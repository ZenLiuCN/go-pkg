package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/urfave/cli/v3"
)

// EsbuildConfig 定义 esbuild 的 JSON 配置结构
type EsbuildConfig struct {
	Color       string            `json:"color"`
	LogLevel    string            `json:"logLevel"`
	LogLimit    int               `json:"logLimit"`
	LogOverride map[string]string `json:"logOverride"`
	AbsPaths    bool              `json:"absPaths"`

	Sourcemap      string `json:"sourcemap"`
	SourceRoot     string `json:"sourceRoot"`
	SourcesContent string `json:"sourcesContent"`

	Target    string          `json:"target"`
	Engines   []api.Engine    `json:"engines"`
	Supported map[string]bool `json:"supported"`

	MangleProps       string                 `json:"mangleProps"`
	ReserveProps      string                 `json:"reserveProps"`
	MangleQuoted      string                 `json:"mangleQuoted"`
	MangleCache       map[string]interface{} `json:"mangleCache"`
	Drop              []string               `json:"drop"`
	DropLabels        []string               `json:"dropLabels"`
	MinifyWhitespace  bool                   `json:"minifyWhitespace"`
	MinifyIdentifiers bool                   `json:"minifyIdentifiers"`
	MinifySyntax      bool                   `json:"minifySyntax"`
	LineLimit         int                    `json:"lineLimit"`
	Charset           string                 `json:"charset"`
	TreeShaking       string                 `json:"treeShaking"`
	IgnoreAnnotations bool                   `json:"ignoreAnnotations"`
	LegalComments     string                 `json:"legalComments"`

	JSX             string `json:"jsx"`
	JSXFactory      string `json:"jsxFactory"`
	JSXFragment     string `json:"jsxFragment"`
	JSXImportSource string `json:"jsxImportSource"`
	JSXDev          bool   `json:"jsxDev"`
	JSXSideEffects  bool   `json:"jsxSideEffects"`

	Define    map[string]string `json:"define"`
	Pure      []string          `json:"pure"`
	KeepNames bool              `json:"keepNames"`

	GlobalName        string            `json:"globalName"`
	Bundle            bool              `json:"bundle"`
	PreserveSymlinks  bool              `json:"preserveSymlinks"`
	Splitting         bool              `json:"splitting"`
	Outfile           string            `json:"outfile"`
	Metafile          bool              `json:"metafile"`
	Outdir            string            `json:"outdir"`
	Outbase           string            `json:"outbase"`
	AbsWorkingDir     string            `json:"absWorkingDir"`
	Platform          string            `json:"platform"`
	Format            string            `json:"format"`
	External          []string          `json:"external"`
	Packages          string            `json:"packages"`
	Alias             map[string]string `json:"alias"`
	MainFields        []string          `json:"mainFields"`
	Conditions        []string          `json:"conditions"`
	Loader            map[string]string `json:"loader"`
	ResolveExtensions []string          `json:"resolveExtensions"`
	Tsconfig          string            `json:"tsconfig"`
	TsconfigRaw       string            `json:"tsconfigRaw"`
	OutExtension      map[string]string `json:"outExtension"`
	PublicPath        string            `json:"publicPath"`
	Inject            []string          `json:"inject"`
	Banner            map[string]string `json:"banner"`
	Footer            map[string]string `json:"footer"`
	NodePaths         []string          `json:"nodePaths"`

	EntryNames  string   `json:"entryNames"`
	ChunkNames  string   `json:"chunkNames"`
	AssetNames  string   `json:"assetNames"`
	EntryPoints []string `json:"entryPoints"`

	Stdin          *api.StdinOptions `json:"stdin"`
	Write          bool              `json:"write"`
	AllowOverwrite bool              `json:"allowOverwrite"`
	Watch          bool              `json:"watch"`
}

func esbuild() *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "bundle and minify JavaScript/TypeScript with esbuild Go API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "path to esbuild config file (JSON)",
			},
			&cli.StringSliceFlag{
				Name:    "entry",
				Aliases: []string{"e"},
				Usage:   "entry point files",
			},
			&cli.StringFlag{
				Name:    "outfile",
				Aliases: []string{"o"},
				Usage:   "output file (for single entry)",
			},
			&cli.StringFlag{
				Name:    "outdir",
				Aliases: []string{"d"},
				Usage:   "output directory (for multiple entries)",
			},
			&cli.BoolFlag{
				Name:    "bundle",
				Aliases: []string{"b"},
				Usage:   "bundle all dependencies",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "minify",
				Aliases: []string{"m"},
				Usage:   "minify output (sets all minify options)",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:  "minify-whitespace",
				Usage: "minify whitespace",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "minify-identifiers",
				Usage: "minify identifiers",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "minify-syntax",
				Usage: "minify syntax",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "sourcemap",
				Usage: "generate source maps (true, false, inline, external, both)",
				Value: "false",
			},
			&cli.BoolFlag{
				Name:    "watch",
				Aliases: []string{"w"},
				Usage:   "watch files and rebuild on changes",
				Value:   false,
			},
			&cli.StringFlag{
				Name:  "platform",
				Usage: "platform target (browser, node, neutral)",
				Value: "browser",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format (iife, cjs, esm)",
				Value: "iife",
			},
			&cli.StringFlag{
				Name:  "target",
				Usage: "language target (e.g. es2015, es2020)",
				Value: "es2015",
			},
			&cli.StringFlag{
				Name:    "tsconfig",
				Aliases: []string{"t"},
				Usage:   "path to tsconfig.json",
			},
			&cli.StringFlag{
				Name:  "jsx",
				Usage: "jsx mode (transform, preserve, automatic)",
			},
			&cli.StringFlag{
				Name:  "jsx-factory",
				Usage: "jsx factory function",
			},
			&cli.StringFlag{
				Name:  "jsx-fragment",
				Usage: "jsx fragment function",
			},
			&cli.StringFlag{
				Name:  "jsx-import-source",
				Usage: "jsx import source",
			},
			&cli.BoolFlag{
				Name:  "jsx-dev",
				Usage: "enable jsx dev mode",
			},
			&cli.StringSliceFlag{
				Name:  "external",
				Usage: "external packages",
			},
			&cli.StringFlag{
				Name:  "global-name",
				Usage: "global name for iife format",
			},
			&cli.BoolFlag{
				Name:  "splitting",
				Usage: "enable code splitting",
			},
			&cli.BoolFlag{
				Name:  "metafile",
				Usage: "generate metafile",
			},
			&cli.StringSliceFlag{
				Name:  "define",
				Usage: "define global constants",
			},
			&cli.StringSliceFlag{
				Name:  "loader",
				Usage: "configure loader for file extensions (ext:loader)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var config EsbuildConfig

			// 如果提供了配置文件，从文件加载配置
			if configFile := cmd.String("config"); configFile != "" {
				configData, err := ioutil.ReadFile(configFile)
				if err != nil {
					return fmt.Errorf("failed to read config file: %v", err)
				}

				if err := json.Unmarshal(configData, &config); err != nil {
					return fmt.Errorf("failed to parse config file: %v", err)
				}
			}

			// 用命令行参数覆盖配置文件中的设置
			if entries := cmd.StringSlice("entry"); len(entries) > 0 {
				config.EntryPoints = entries
			}
			if outfile := cmd.String("outfile"); outfile != "" {
				config.Outfile = outfile
			}
			if outdir := cmd.String("outdir"); outdir != "" {
				config.Outdir = outdir
			}
			if cmd.IsSet("bundle") {
				config.Bundle = cmd.Bool("bundle")
			}
			if cmd.IsSet("minify") {
				minify := cmd.Bool("minify")
				config.MinifyWhitespace = minify
				config.MinifyIdentifiers = minify
				config.MinifySyntax = minify
			}
			if cmd.IsSet("minify-whitespace") {
				config.MinifyWhitespace = cmd.Bool("minify-whitespace")
			}
			if cmd.IsSet("minify-identifiers") {
				config.MinifyIdentifiers = cmd.Bool("minify-identifiers")
			}
			if cmd.IsSet("minify-syntax") {
				config.MinifySyntax = cmd.Bool("minify-syntax")
			}
			if cmd.IsSet("sourcemap") {
				config.Sourcemap = cmd.String("sourcemap")
			}
			if cmd.IsSet("watch") {
				config.Watch = cmd.Bool("watch")
			}
			if platform := cmd.String("platform"); platform != "" {
				config.Platform = platform
			}
			if format := cmd.String("format"); format != "" {
				config.Format = format
			}
			if target := cmd.String("target"); target != "" {
				config.Target = target
			}
			if tsconfig := cmd.String("tsconfig"); tsconfig != "" {
				config.Tsconfig = tsconfig
			}
			if jsx := cmd.String("jsx"); jsx != "" {
				config.JSX = jsx
			}
			if jsxFactory := cmd.String("jsx-factory"); jsxFactory != "" {
				config.JSXFactory = jsxFactory
			}
			if jsxFragment := cmd.String("jsx-fragment"); jsxFragment != "" {
				config.JSXFragment = jsxFragment
			}
			if jsxImportSource := cmd.String("jsx-import-source"); jsxImportSource != "" {
				config.JSXImportSource = jsxImportSource
			}
			if cmd.IsSet("jsx-dev") {
				config.JSXDev = cmd.Bool("jsx-dev")
			}
			if external := cmd.StringSlice("external"); len(external) > 0 {
				config.External = external
			}
			if globalName := cmd.String("global-name"); globalName != "" {
				config.GlobalName = globalName
			}
			if cmd.IsSet("splitting") {
				config.Splitting = cmd.Bool("splitting")
			}
			if cmd.IsSet("metafile") {
				config.Metafile = cmd.Bool("metafile")
			}
			if defines := cmd.StringSlice("define"); len(defines) > 0 {
				config.Define = make(map[string]string)
				for _, d := range defines {
					parts := strings.SplitN(d, "=", 2)
					if len(parts) == 2 {
						config.Define[parts[0]] = parts[1]
					} else {
						config.Define[parts[0]] = "true"
					}
				}
			}
			if loaders := cmd.StringSlice("loader"); len(loaders) > 0 {
				config.Loader = make(map[string]string)
				for _, l := range loaders {
					parts := strings.SplitN(l, ":", 2)
					if len(parts) == 2 {
						config.Loader[parts[0]] = parts[1]
					}
				}
			}

			// 验证必要参数
			if len(config.EntryPoints) == 0 {
				return fmt.Errorf("at least one entry point must be specified")
			}
			if config.Outfile == "" && config.Outdir == "" {
				return fmt.Errorf("either outfile or outdir must be specified")
			}

			// 构建 esbuild 选项
			buildOptions := api.BuildOptions{
				EntryPoints:       config.EntryPoints,
				Bundle:            config.Bundle,
				MinifyWhitespace:  config.MinifyWhitespace,
				MinifyIdentifiers: config.MinifyIdentifiers,
				MinifySyntax:      config.MinifySyntax,
				Platform:          parsePlatform(config.Platform),
				Format:            parseFormat(config.Format),
				Target:            parseTarget(config.Target),
				Tsconfig:          config.Tsconfig,
				Write:             true,
				Metafile:          config.Metafile,
				Splitting:         config.Splitting,
				GlobalName:        config.GlobalName,
				JSX:               parseJSX(config.JSX),
				JSXFactory:        config.JSXFactory,
				JSXFragment:       config.JSXFragment,
				JSXImportSource:   config.JSXImportSource,
				JSXDev:            config.JSXDev,
				External:          config.External,
				Define:            config.Define,
				Sourcemap:         parseSourceMap(config.Sourcemap),
			}

			// 设置加载器
			if len(config.Loader) > 0 {
				buildOptions.Loader = make(map[string]api.Loader)
				for ext, loader := range config.Loader {
					buildOptions.Loader[ext] = parseLoader(loader)
				}
			}

			if config.Outfile != "" {
				buildOptions.Outfile = config.Outfile
			} else {
				buildOptions.Outdir = config.Outdir
			}

			// 执行构建
			if config.Watch {
				return runEsbuildWatch(ctx, buildOptions)
			}
			return runEsbuildOnce(buildOptions)
		},
	}
}

// 辅助函数：解析平台类型
func parsePlatform(platform string) api.Platform {
	switch platform {
	case "node":
		return api.PlatformNode
	case "neutral":
		return api.PlatformNeutral
	default:
		return api.PlatformBrowser
	}
}

// 辅助函数：解析格式类型
func parseFormat(format string) api.Format {
	switch format {
	case "cjs":
		return api.FormatCommonJS
	case "esm":
		return api.FormatESModule
	default:
		return api.FormatIIFE
	}
}

// 辅助函数：解析目标类型
func parseTarget(target string) api.Target {
	switch target {
	case "es2024":
		return api.ES2024
	case "es2023":
		return api.ES2023
	case "es2022":
		return api.ES2022
	case "es2021":
		return api.ES2021
	case "es2020":
		return api.ES2020
	case "es2019":
		return api.ES2019
	case "es2018":
		return api.ES2018
	case "es2017":
		return api.ES2017
	case "es2016":
		return api.ES2016
	default:
		return api.ES2015
	}
}

// 辅助函数：解析JSX模式
func parseJSX(jsx string) api.JSX {
	switch jsx {
	case "preserve":
		return api.JSXPreserve
	case "automatic":
		return api.JSXAutomatic
	default:
		return api.JSXTransform
	}
}

// 辅助函数：解析sourcemap选项
func parseSourceMap(sourcemap string) api.SourceMap {
	switch sourcemap {
	case "inline":
		return api.SourceMapInline
	case "external":
		return api.SourceMapExternal
	case "both":
		return api.SourceMapInlineAndExternal
	case "true":
		return api.SourceMapInline
	default:
		return api.SourceMapNone
	}
}

// 辅助函数：解析loader类型
func parseLoader(loader string) api.Loader {
	switch loader {
	case "js":
		return api.LoaderJS
	case "jsx":
		return api.LoaderJSX
	case "ts":
		return api.LoaderTS
	case "tsx":
		return api.LoaderTSX
	case "css":
		return api.LoaderCSS
	case "localCss":
		return api.LoaderLocalCSS
	case "globalCss":
		return api.LoaderGlobalCSS
	case "json":
		return api.LoaderJSON
	case "text":
		return api.LoaderText
	case "empty":
		return api.LoaderEmpty
	case "base64":
		return api.LoaderBase64
	case "dataurl":
		return api.LoaderDataURL
	case "file":
		return api.LoaderFile
	case "binary":
		return api.LoaderBinary
	case "copy":
		return api.LoaderCopy
	case "default":
		return api.LoaderDefault
	default:
		return api.LoaderNone
	}
}

// runEsbuildOnce 执行一次 esbuild 构建
func runEsbuildOnce(options api.BuildOptions) error {
	result := api.Build(options)
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			log.Printf("Error: %s", err.Text)
		}
		return fmt.Errorf("build failed with %d errors", len(result.Errors))
	}

	if len(result.Warnings) > 0 {
		for _, warn := range result.Warnings {
			log.Printf("Warning: %s", warn.Text)
		}
	}

	if options.Metafile && len(result.Metafile) > 0 {
		metafilePath := "meta.json"
		if options.Outdir != "" {
			metafilePath = filepath.Join(options.Outdir, "meta.json")
		}
		if err := ioutil.WriteFile(metafilePath, []byte(result.Metafile), 0644); err != nil {
			log.Printf("Failed to write metafile: %v", err)
		} else {
			log.Printf("Metafile written to %s", metafilePath)
		}
	}

	log.Println("Build completed successfully")
	return nil
}

// runEsbuildWatch 监控文件变化并执行 esbuild
func runEsbuildWatch(ctx context.Context, options api.BuildOptions) error {
	// 创建构建上下文
	buildCtx, err := api.Context(options)
	if err != nil {
		return fmt.Errorf("failed to create build context: %v", err)
	}
	defer buildCtx.Dispose()

	// 第一次构建
	result := buildCtx.Rebuild()
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			log.Printf("Error: %s", err.Text)
		}
		return fmt.Errorf("initial build failed with %d errors", len(result.Errors))
	}

	// 启动 watch 模式
	watchErr := buildCtx.Watch(api.WatchOptions{
		Delay: 100,
	})

	if watchErr != nil {
		return fmt.Errorf("failed to start watch mode: %v", watchErr)
	}

	log.Println("Watching for changes... (press Ctrl+C to stop)")

	// 监听上下文取消信号
	<-ctx.Done()
	log.Println("Stopping watch mode...")

	return nil
}
