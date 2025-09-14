package commands

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/ZenLiuCN/fn"
	. "github.com/urfave/cli/v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	client = &http.Client{Timeout: 60 * time.Minute}
)

const (
	Mirror  = "https://registry.npmjs.org"
	Mirror2 = "https://mirrors.cloud.tencent.com/nexus/repository/maven-public"
)

func npm() *Command {
	return &Command{
		Name:  "npm",
		Usage: "fetch a npm package archive",
		Flags: []Flag{
			&StringFlag{Name: "output", Aliases: []string{"o"}, DefaultText: "working directory"},
			&StringFlag{
				Name:    "mirror",
				Aliases: []string{"m"},
				Usage:   "mirror site",
			},
		},
		Arguments: []Argument{&StringArgs{
			Name:      "package",
			UsageText: "name with optional version value delimited by @",
			Max:       -1,
			Min:       1,
		}},
		Action: func(ctx context.Context, cmd *Command) (err error) {
			m := Mirror
			mx := cmd.String("mirror")
			if mx != "" {
				m = mx
			}
			o := cmd.String("output")
			if o == "" {
				o = fn.Panic1(os.Getwd())
			}
			if f, e := isFile(o); !e || f {
				return fmt.Errorf("%s should be a folder", o)
			}
			for _, pkg := range cmd.StringArgs("package") {
				i := strings.LastIndexByte(pkg, '@')
				if i <= 0 {
					err = fetchNPM(o, m, pkg, "")
					if err != nil {
						return
					}
				} else {
					err = fetchNPM(o, m, pkg[0:i], pkg[i+1:])
					if err != nil {
						return
					}
				}
			}
			return err
		}}
}
func tsd() *Command {
	return &Command{
		Name:  "types",
		Usage: "extract typescript defines from npm package archive or extracted folder",
		Flags: []Flag{
			&StringFlag{Name: "output", Aliases: []string{"o"}, DefaultText: "working directory"},
		},
		Arguments: []Argument{&StringArgs{
			Name:      "package",
			UsageText: "package files or folders",
			Max:       -1,
			Min:       1,
		}},
		Action: func(ctx context.Context, cmd *Command) (err error) {
			o := cmd.String("output")
			if o == "" {
				o = fn.Panic1(os.Getwd())
			}
			if o, err = filepath.Abs(o); err != nil {
				return err
			}

			if f, e := isFile(o); !e {
				_ = os.MkdirAll(o, os.ModePerm)
			} else if f {
				return fmt.Errorf("%s should be a folder", o)
			}
			for _, pkg := range cmd.StringArgs("package") {
				if pkg, err = filepath.Abs(pkg); err != nil {
					return err
				}
				if f, e := isFile(pkg); !e {
					return fmt.Errorf("%s missing", pkg)
				} else if f {
					if err = typingFromFile(pkg, o); err != nil {
						return err
					}
				} else {
					if err = typingFromFolder(pkg, o); err != nil {
						return err
					}
				}
			}
			return err
		},
	}
}

func typingFromFile(pkg string, o string) error {
	file, err := os.Open(pkg)
	if err != nil {
		return fmt.Errorf("fail to open: %w", err)
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	totalFiles := 0
	extractedFiles := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // 文件结束
		}
		if err != nil {
			return fmt.Errorf("read archive fail: %w", err)
		}
		totalFiles++
		// 只处理 .ts.d 文件
		if strings.HasSuffix(header.Name, ".d.ts") {
			extractedFiles++
			targetPath := filepath.Join(o, strings.ReplaceAll(header.Name, "package/", ""))
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create file: %s: %w", targetPath, err)
			}
			// 复制文件内容
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file: %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}
	return nil
}
func typingFromFolder(pkg string, o string) error {
	return filepath.Walk(pkg, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(info.Name()) != ".d" {
			return nil
		}
		base := filepath.Base(path)
		if len(base) > 3 && base[len(base)-4:] == ".ts.d" {
			relPath, err := filepath.Rel(pkg, path)
			if err != nil {
				return fmt.Errorf("relative path error: %w", err)
			}
			destPath := filepath.Join(o, relPath)
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("create folder fail: %w", err)
			}
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("copy file : %w", err)
			}
		}
		return nil
	})
}
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
func fetchNPM(out, mirror, pkg, version string) (err error) {
	defer func() {
		switch x := recover().(type) {
		case error:
			err = x
		case string:
			err = fmt.Errorf("%s", x)
		}
	}()
	if version == "" {
		log.Println("read latest version")
		r := fn.Panic1(client.Get(fmt.Sprintf("%s/%s", mirror, pkg)))
		defer r.Body.Close()
		if r.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(r.Body)
			panic(fmt.Errorf("fetch last version failed (%d): %s", r.StatusCode, string(body)))
		}
		var data struct {
			DistTags struct {
				Latest string `json:"latest"`
			} `json:"dist-tags"`
		}

		fn.Panic(json.NewDecoder(r.Body).Decode(&data))
		version = data.DistTags.Latest
	}
	if version == "" {
		panic(fmt.Errorf("missing version for %s", pkg))
	}
	baseName := path.Base(pkg)
	log.Printf("download %s %s", pkg, version)
	r := fn.Panic1(client.Get(fmt.Sprintf("%s/%s/-/%s-%s.tgz", mirror, pkg, baseName, version)))
	if r.StatusCode != http.StatusOK {
		panic(fmt.Errorf("fetch last version failed:%d %s", r.StatusCode, r.Status))
	}
	safePkgName := strings.ReplaceAll(pkg, "/", "_")
	filename := fmt.Sprintf("%s/%s-%s.tgz", out, safePkgName, version)
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(r.Body)
		panic(fmt.Errorf("download failed (%d): %s", r.StatusCode, string(body)))
	}
	file, err := os.Create(filename)
	if err != nil {
		panic(fmt.Errorf("create file: %w", err))
	}
	defer file.Close()
	if _, err = io.Copy(file, r.Body); err != nil {
		_ = os.Remove(filename)
		panic(fmt.Errorf("save content: %w", err))
	}
	log.Printf("store %s %s to %s", pkg, version, file.Name())
	return
}
func isFile(name string) (file bool, exist bool) {
	s, err := os.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, false
	}
	if s.IsDir() {
		return false, true
	}
	return true, true
}
func mvn() *Command {
	return &Command{
		Name:  "mvn",
		Usage: "fetch a maven package archive",
		Flags: []Flag{
			&StringFlag{Name: "output", Aliases: []string{"o"}, DefaultText: "working directory"},
			&StringFlag{
				Name:    "mirror",
				Aliases: []string{"m"},
				Usage:   "mirror site",
			},
		},
		Arguments: []Argument{&StringArgs{
			Name:      "package",
			UsageText: "name with optional version value delimited by @",
			Max:       -1,
			Min:       1,
		}},
		Action: func(ctx context.Context, cmd *Command) (err error) {
			m := Mirror2
			mx := cmd.String("mirror")
			if mx != "" {
				m = mx
			}
			o := cmd.String("output")
			if o == "" {
				o = fn.Panic1(os.Getwd())
			}
			f, e := isFile(o)
			if f || !e {
				return fmt.Errorf("%s should be a folder", o)
			}
			for _, pkg := range cmd.StringArgs("package") {
				err = fetchMaven(o, m, pkg)
				if err != nil {
					return
				}
			}
			return err
		}}
}

func fetchMaven(out, mirror, pkg string) error {
	parts, packaging, err := parseDependency(pkg)
	if err != nil {
		return err
	}
	group := parts[0]
	artifact := parts[1]
	version := parts[2]
	if version == "" {
		log.Printf("fetch lastest version of %s:%s", group, artifact)
		version, err = getLatestVersion(mirror, group, artifact)
		if err != nil {
			return err
		}
	}
	classifier := ""
	if len(parts) >= 4 {
		classifier = parts[3]
	}
	filename := artifact + "-" + version
	if classifier != "" {
		filename += "-" + classifier
	}
	filename += "." + packaging
	groupPath := strings.ReplaceAll(group, ".", "/")
	jarPath := fmt.Sprintf("%s/%s/%s/%s", groupPath, artifact, version, filename)

	downloadURL := strings.TrimSuffix(mirror, "/") + "/" + jarPath
	outputPath := filepath.Join(out, filename)
	log.Printf("download  %s:%s:%s:%s", group, artifact, version, classifier)
	host := fn.Panic1(url.Parse(downloadURL)).Host
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Host", host)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36")
	r, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch error: %w", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch error: %s", r.Status)
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create file error: %w", err)
	}
	defer file.Close()
	if _, err = io.Copy(file, r.Body); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("download file : %w", err)
	}
	log.Printf("store %s %s to %s", pkg, version, file.Name())
	return nil
}
func parseDependency(dependency string) ([]string, string, error) {
	// 提取packaging部分
	packaging := "jar"
	if atIndex := strings.LastIndex(dependency, "@"); atIndex != -1 {
		packaging = dependency[atIndex+1:]
		dependency = dependency[:atIndex]
	}
	// 切分依赖项
	parts := strings.Split(dependency, ":")
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("invalid segment: %s", dependency)
	}
	// 验证必需字段
	if parts[0] == "" || parts[1] == "" {
		return nil, "", fmt.Errorf("invalid segment: %s", dependency)
	}
	return parts, packaging, nil
}
func getLatestVersion(mirror, group, artifact string) (string, error) {
	groupPath := strings.ReplaceAll(group, ".", "/")
	metadataURL := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", strings.TrimSuffix(mirror, "/"), groupPath, artifact)
	host := fn.Panic1(url.Parse(metadataURL)).Host
	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/xhtml+xml,application/xml;")
	req.Header.Add("Host", host)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch metadata: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata: %w", err)
	}

	// 解析maven-metadata.xml获取最新版本
	type Metadata struct {
		Versioning struct {
			Latest  string `xml:"latest"`
			Release string `xml:"release"`
		} `xml:"versioning"`
	}

	var metadata Metadata
	if err := xml.Unmarshal(body, &metadata); err != nil {
		return "", fmt.Errorf("failed to parse metadata: %w", err)
	}

	if metadata.Versioning.Latest != "" {
		return metadata.Versioning.Latest, nil
	}
	if metadata.Versioning.Release != "" {
		return metadata.Versioning.Release, nil
	}

	return "", fmt.Errorf("no latest version found in metadata")
}
