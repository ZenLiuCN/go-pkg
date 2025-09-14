package commands

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	. "github.com/urfave/cli/v3"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func httpd() *Command {
	return &Command{
		Name:  "httpd",
		Usage: "http local server",
		Flags: []Flag{
			&IntFlag{
				Name:        "port",
				Aliases:     []string{"p"},
				DefaultText: "8090",
				Value:       8090,
			},
			&StringFlag{
				Name:        "ip",
				Aliases:     []string{"i"},
				DefaultText: "*",
				Value:       "*",
			},
			&StringSliceFlag{
				Name:        "watch",
				Aliases:     []string{"w"},
				Usage:       "file extensions to watch for hot reload (e.g. .html,.js,.css)",
				DefaultText: ".html",
				Value:       []string{".html"},
			},
			&StringSliceFlag{
				Name:        "inject",
				Aliases:     []string{"j"},
				Usage:       "file extensions to inject hot reload script (e.g. .html,.php)",
				DefaultText: ".html",
				Value:       []string{".html"},
			},
		},
		Arguments: []Argument{
			&StringArgs{
				Name:      "dir",
				UsageText: "file folder",
				Min:       0,
			},
		},
		Action: func(ctx context.Context, cmd *Command) error {
			port := cmd.Int("port")
			ip := cmd.String("ip")
			watchExts := cmd.StringSlice("watch")
			injectExts := cmd.StringSlice("inject")
			dirs := cmd.StringArgs("dir")
			var dir string
			if len(dirs) > 0 {
				dir = dirs[0]
			} else {
				dir, _ = os.Getwd()
			}
			// 检查目录是否存在
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("directory %s does not exist", dir)
			}
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %v", err)
			}
			// 创建文件系统处理器
			fs := http.FileServer(http.Dir(absDir))
			// 创建带热重载的处理器
			handler := &hotReloadHandler{
				fs:         fs,
				dir:        absDir,
				watcher:    setupWatcher(absDir, watchExts),
				clients:    make(map[chan []byte]bool),
				mu:         sync.Mutex{},
				shutdown:   make(chan struct{}),
				watchExts:  watchExts,
				injectExts: injectExts,
			}

			// 启动文件监听
			go handler.watchFiles()

			// 设置HTTP服务器
			addr := fmt.Sprintf("%s:%d", ip, port)
			server := &http.Server{
				Addr:    addr,
				Handler: handler,
			}

			log.Printf("Serving %s on http://%s\n", absDir, addr)
			log.Println("Press Ctrl+C to stop")

			// 启动服务器
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("Server error: %v", err)
				}
			}()

			// 等待上下文取消信号
			<-ctx.Done()

			// 关闭服务器和文件监听
			handler.close()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		},
	}
}

// hotReloadHandler 处理HTTP请求并支持热重载
type hotReloadHandler struct {
	fs         http.Handler
	dir        string
	watcher    *fsnotify.Watcher
	clients    map[chan []byte]bool
	mu         sync.Mutex
	shutdown   chan struct{}
	watchExts  []string // 需要监听的文件扩展名
	injectExts []string // 需要注入热重载脚本的文件扩展名
}

// ServeHTTP 处理HTTP请求
func (h *hotReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 检查是否需要注入热重载脚本
	for _, ext := range h.injectExts {
		if strings.HasSuffix(r.URL.Path, ext) {
			// 先让文件服务器处理请求
			rec := &responseRecorder{ResponseWriter: w}
			h.fs.ServeHTTP(rec, r)

			// 检查是否成功返回了内容
			if rec.status == http.StatusOK {
				// 根据内容类型决定是否注入脚本
				contentType := rec.Header().Get("Content-Type")
				if strings.Contains(contentType, "text/html") ||
					strings.Contains(contentType, "application/xhtml+xml") {
					// 注入热重载脚本
					body := rec.buf.String()
					reloadScript := `
						<script>
							(function() {
								const evtSource = new EventSource("/_hotreload");
								evtSource.onmessage = function(e) {
									if (e.data === "reload") {
										console.log("Reloading page...");
										location.reload();
									}
								};
								evtSource.onerror = function() {
									console.log("EventSource error. Closing connection.");
									evtSource.close();
								};
							})();
						</script>
					`
					// 在</body>标签前插入脚本，如果没有</body>则追加到末尾
					if strings.Contains(body, "</body>") {
						body = strings.Replace(body, "</body>", reloadScript+"</body>", 1)
					} else {
						body += reloadScript
					}
					w.Header().Set("Content-Length", fmt.Sprint(len(body)))
					w.WriteHeader(rec.status)
					_, _ = w.Write([]byte(body))
					return
				}
			}
		}
	}

	// 处理热重载事件源请求
	if r.URL.Path == "/_hotreload" {
		h.handleHotReload(w, r)
		return
	}

	// 其他文件正常处理
	h.fs.ServeHTTP(w, r)
}

// handleHotReload 处理SSE连接
func (h *hotReloadHandler) handleHotReload(w http.ResponseWriter, r *http.Request) {
	// 设置SSE头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 创建消息通道
	messageChan := make(chan []byte)

	// 注册客户端
	h.mu.Lock()
	h.clients[messageChan] = true
	h.mu.Unlock()

	// 确保客户端退出时注销
	defer func() {
		h.mu.Lock()
		delete(h.clients, messageChan)
		h.mu.Unlock()
		close(messageChan)
	}()

	// 保持连接打开
	flusher, _ := w.(http.Flusher)
	for {
		select {
		case msg := <-messageChan:
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-h.shutdown:
			return
		}
	}
}

// watchFiles 监听文件变化
func (h *hotReloadHandler) watchFiles() {
	for {
		select {
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}
			// 检查是否是监听的文件类型
			for _, ext := range h.watchExts {
				if event.Has(fsnotify.Write) && strings.HasSuffix(event.Name, ext) {
					log.Printf("File changed: %s", event.Name)
					h.notifyClients("reload")
					break
				}
			}
		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		case <-h.shutdown:
			return
		}
	}
}

// notifyClients 通知所有客户端重新加载
func (h *hotReloadHandler) notifyClients(msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client <- []byte(msg):
		default:
			// 如果客户端无法接收消息，跳过
		}
	}
}

// close 关闭资源
func (h *hotReloadHandler) close() {
	close(h.shutdown)
	if h.watcher != nil {
		h.watcher.Close()
	}
}

// setupWatcher 设置文件监听器
func setupWatcher(dir string, watchExts []string) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	// 递归添加目录监听
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 忽略隐藏目录（以.开头）
			if filepath.Base(path)[0] == '.' {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to watch directory: %v", err)
	}

	return watcher
}

// responseRecorder 用于捕获文件服务器的响应
type responseRecorder struct {
	http.ResponseWriter
	buf    strings.Builder
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.buf.Write(b)
}
