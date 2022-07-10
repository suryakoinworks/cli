package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bimalabs/cli/generated/generator"
	bima "github.com/bimalabs/framework/v4"
	"github.com/bimalabs/framework/v4/configs"
	"github.com/bimalabs/framework/v4/generators"
	"github.com/bimalabs/framework/v4/parsers"
	"github.com/bimalabs/framework/v4/utils"
	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/copier"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
	"github.com/vito/go-interact/interact"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v2"
	"mvdan.cc/sh/interp"
	"mvdan.cc/sh/syntax"
)

var (
	Version     = "v1.1.9"
	SpinerIndex = 9
	Duration    = 77 * time.Millisecond

	Env = `APP_DEBUG=true
APP_PORT=7777
GRPC_PORT=1717
APP_NAME=%s
APP_SECRET=%s
    `

	Adapter = `package adapters

import (
    "context"

    "github.com/bimalabs/framework/v4/paginations"
    "github.com/vcraescu/go-paginator/v2"
)

type %s struct {
}

func (a *%s) CreateAdapter(ctx context.Context, paginator paginations.Pagination) paginator.Adapter {
    // TODO

    return nil
}
`

	Driver = `package drivers

import (
    "gorm.io/gorm"
)

type %s string

func (_ %s) Connect(host string, port int, user string, password string, dbname string, debug bool) *gorm.DB {
    // TODO

    return nil
}

func (m %s) Name() string {
	return string(m)
}
`

	Route = `package routes

import (
    "net/http"

    "github.com/bimalabs/framework/v4/middlewares"
    "google.golang.org/grpc"
)

type %s struct {
}

func (r *%s) Path() string {
    return "/%s"
}

func (r *%s) Method() string {
    return http.MethodGet
}

func (r *%s) SetClient(client *grpc.ClientConn) {
    // TODO
}

func (r *%s) Middlewares() []middlewares.Middleware {
    // TODO

    return nil
}

func (r *%s) Handle(response http.ResponseWriter, request *http.Request, params map[string]string) {
    // TODO
}
`

	Middleware = `package middlewares

import (
    "net/http"
)

type %s struct {
}

func (m *%s) Attach(request *http.Request, response http.ResponseWriter) bool {
    // TODO

    return false
}

func (m *%s) Priority() int {
    return 0
}
`
)

func main() {
	var file string
	app := &cli.App{
		Name:                 "Bima Cli",
		Usage:                "Bima Framework Toolkit",
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Name:    "create",
				Aliases: []string{"new"},
				Usage:   "bima create <command>",
				Subcommands: []*cli.Command{
					{
						Name:    "project",
						Aliases: []string{"app"},
						Usage:   "bima create app <name>",
						Action: func(cCtx *cli.Context) error {
							name := cCtx.Args().First()
							if name == "" {
								fmt.Println("Usage: bima create app <name>")

								return nil
							}

							progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
							progress.Suffix = " Creating new application... "
							progress.Start()

							err := create(name)
							if err == nil {
								progress.Stop()
								fmt.Printf("%s application created\n", color.New(color.FgGreen).Sprint(strings.Title(name)))

								util := color.New(color.Bold)

								fmt.Print("Move to ")
								util.Print(name)
								fmt.Print(" folder and type ")
								util.Println("bima run")
							}

							progress.Stop()

							return err
						},
					},
					{
						Name:    "middleware",
						Aliases: []string{"mid"},
						Usage:   "bima create middleware <name>",
						Action: func(cCtx *cli.Context) error {
							name := cCtx.Args().First()
							if name == "" {
								fmt.Println("Usage: bima create middleware <name>")

								return nil
							}

							progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
							progress.Suffix = " Creating middleware... "
							progress.Start()
							time.Sleep(1 * time.Second)

							wd, err := os.Getwd()
							if err != nil {
								progress.Stop()

								return err
							}

							err = os.MkdirAll(fmt.Sprintf("%s/middlewares", wd), 0755)
							if err != nil {
								progress.Stop()

								return err
							}

							f, err := os.Create(fmt.Sprintf("%s/middlewares/%s.go", wd, strings.ToLower(name)))
							if err != nil {
								progress.Stop()

								return err
							}

							name = strings.Title(name)
							_, err = f.WriteString(fmt.Sprintf(Middleware, name, name, name))
							if err != nil {
								progress.Stop()

								return err
							}

							f.Sync()
							f.Close()

							if err := clean(); err != nil {
								progress.Stop()
								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							progress.Stop()
							fmt.Printf("Middleware %s created\n", color.New(color.FgGreen).Sprint(name))

							return nil
						},
					},
					{
						Name:    "driver",
						Aliases: []string{"dvr"},
						Usage:   "bima create driver <name>",
						Action: func(cCtx *cli.Context) error {
							name := cCtx.Args().First()
							if name == "" {
								fmt.Println("Usage: bima create driver <name>")

								return nil
							}

							progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
							progress.Suffix = " Creating database driver... "
							progress.Start()
							time.Sleep(1 * time.Second)

							wd, err := os.Getwd()
							if err != nil {
								progress.Stop()

								return err
							}

							err = os.MkdirAll(fmt.Sprintf("%s/drivers", wd), 0755)
							if err != nil {
								progress.Stop()

								return err
							}

							f, err := os.Create(fmt.Sprintf("%s/drivers/%s.go", wd, strings.ToLower(name)))
							if err != nil {
								progress.Stop()

								return err
							}

							name = strings.Title(name)
							_, err = f.WriteString(fmt.Sprintf(Driver, name, name, name))
							if err != nil {
								progress.Stop()

								return err
							}

							f.Sync()
							f.Close()

							if err := clean(); err != nil {
								progress.Stop()
								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							progress.Stop()
							fmt.Printf("Driver %s created\n", color.New(color.FgGreen).Sprint(name))

							return nil
						},
					},
					{
						Name:    "adapter",
						Aliases: []string{"adp"},
						Usage:   "bima create adapter <name>",
						Action: func(cCtx *cli.Context) error {
							name := cCtx.Args().First()
							if name == "" {
								fmt.Println("Usage: bima create adapter <name>")

								return nil
							}

							progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
							progress.Suffix = " Creating pagination adapter... "
							progress.Start()
							time.Sleep(1 * time.Second)

							wd, err := os.Getwd()
							if err != nil {
								progress.Stop()

								return err
							}

							err = os.MkdirAll(fmt.Sprintf("%s/adapters", wd), 0755)
							if err != nil {
								progress.Stop()

								return err
							}

							f, err := os.Create(fmt.Sprintf("%s/adapters/%s.go", wd, strings.ToLower(name)))
							if err != nil {
								progress.Stop()

								return err
							}

							name = strings.Title(name)
							_, err = f.WriteString(fmt.Sprintf(Adapter, name, name))
							if err != nil {
								progress.Stop()

								return err
							}

							f.Sync()
							f.Close()

							if err := clean(); err != nil {
								progress.Stop()

								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							progress.Stop()
							fmt.Printf("Adapter %s created\n", color.New(color.FgGreen).Sprint(name))

							return nil
						},
					},
					{
						Name:    "route",
						Aliases: []string{"rt"},
						Usage:   "bima create route <name>",
						Action: func(cCtx *cli.Context) error {
							name := cCtx.Args().First()
							if name == "" {
								fmt.Println("Usage: bima create route <name>")

								return nil
							}

							progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
							progress.Suffix = " Creating route placeholder... "
							progress.Start()
							time.Sleep(1 * time.Second)

							wd, err := os.Getwd()
							if err != nil {
								progress.Stop()

								return err
							}

							err = os.MkdirAll(fmt.Sprintf("%s/routes", wd), 0755)
							if err != nil {
								progress.Stop()

								return err
							}

							lName := strings.ToLower(name)
							f, err := os.Create(fmt.Sprintf("%s/routes/%s.go", wd, lName))
							if err != nil {
								progress.Stop()

								return err
							}

							name = strings.Title(name)
							_, err = f.WriteString(fmt.Sprintf(Route, name, name, lName, name, name, name, name))
							if err != nil {
								progress.Stop()

								return err
							}

							f.Sync()
							f.Close()

							if err := clean(); err != nil {
								progress.Stop()

								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							progress.Stop()
							fmt.Printf("Route %s created\n", color.New(color.FgGreen).Sprint(name))

							return nil
						},
					},
				},
			},
			{
				Name:    "module",
				Aliases: []string{"mod"},
				Usage:   "module <command>",
				Subcommands: []*cli.Command{
					{
						Name: "add",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:        "file",
								Value:       ".env",
								Usage:       "Config file",
								Destination: &file,
							},
						},
						Aliases: []string{"new"},
						Usage:   "module add <name>",
						Action: func(cCtx *cli.Context) error {
							module := cCtx.Args().First()
							if module == "" {
								fmt.Println("Usage: bima module add <name>")

								return nil
							}

							if err := dump(); err != nil {
								color.New(color.FgRed).Println("Error update DI container")

								return err
							}

							config := configs.Env{}
							env(&config, file, filepath.Ext(file))

							container, err := generator.NewContainer(bima.Generator)
							if err != nil {
								return err
							}

							generator := container.GetBimaModuleGenerator()
							generator.Driver = config.Db.Driver
							generator.ApiVersion = "v1"
							if cCtx.NArg() > 1 {
								generator.ApiVersion = cCtx.Args().Get(1)
							}

							util := color.New(color.FgGreen, color.Bold)

							err = register(generator, util, module)
							if err != nil {
								color.New(color.FgRed).Println(err)
							}

							if err = genproto(); err != nil {
								color.New(color.FgRed).Println("Error generate code from proto files")

								return err
							}

							if err = clean(); err != nil {
								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							if err = dump(); err != nil {
								color.New(color.FgRed).Println("Error update DI container")

								return err
							}

							if err = clean(); err != nil {
								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							return nil
						},
					},
					{
						Name:    "remove",
						Aliases: []string{"rm", "rem"},
						Usage:   "module remove <name>",
						Action: func(cCtx *cli.Context) error {
							module := cCtx.Args().First()
							if module == "" {
								fmt.Println("Usage: bima module add <name>")

								return nil
							}

							if err := dump(); err != nil {
								color.New(color.FgRed).Println("Error cleaning DI container")

								return err
							}

							util := color.New(color.FgGreen, color.Bold)

							unregister(util, module)
							if err := dump(); err != nil {
								color.New(color.FgRed).Println("Error update DI container")

								return err
							}

							if err := clean(); err != nil {
								color.New(color.FgRed).Println("Error cleaning dependencies")

								return err
							}

							return nil
						},
					},
				},
			},
			{
				Name:    "dump",
				Aliases: []string{"dmp"},
				Usage:   "dump",
				Action: func(*cli.Context) error {
					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Generate service container... "
					progress.Start()
					time.Sleep(1 * time.Second)

					err := dump()
					progress.Stop()

					return err
				},
			},
			{
				Name:    "build",
				Aliases: []string{"install", "compile"},
				Usage:   "build <name>",
				Action: func(cCtx *cli.Context) error {
					name := cCtx.Args().First()
					if name == "" {
						fmt.Println("Usage: bima build <name>")

						return nil
					}

					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Bundling application... "
					progress.Start()
					if err := clean(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error cleaning dependencies")

						return err
					}

					if err := dump(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error update DI container")

						return err
					}

					err := build(name, false)
					progress.Stop()

					return err
				},
			},
			{
				Name:    "update",
				Aliases: []string{"upd"},
				Usage:   "update",
				Action: func(*cli.Context) error {
					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Updating dependencies... "
					progress.Start()
					if err := update(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error update dependencies")

						return err
					}

					if err := dump(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error update DI container")

						return err
					}

					progress.Stop()

					return nil
				},
			},
			{
				Name:    "clean",
				Aliases: []string{"cln"},
				Usage:   "clean",
				Action: func(*cli.Context) error {
					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Cleaning dependencies... "
					progress.Start()
					if err := clean(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error cleaning dependencies")

						return err
					}

					if err := dump(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error update DI container")

						return err
					}

					progress.Stop()

					return nil
				},
			},
			{
				Name:    "generate",
				Aliases: []string{"gen", "genproto"},
				Usage:   "generate",
				Action: func(*cli.Context) error {
					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Generating protobuff... "
					progress.Start()
					if err := genproto(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error generate protobuff")

						return err
					}

					if err := clean(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error cleaning dependencies")

						return err
					}

					if err := dump(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error update DI container")

						return err
					}

					progress.Stop()

					return nil
				},
			},
			{
				Name: "run",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "file",
						Aliases:     []string{"f"},
						Value:       ".env",
						Usage:       "Config file",
						Destination: &file,
					},
				},
				Aliases: []string{"rn"},
				Usage:   "run <mode> -f config.json",
				Action: func(cCtx *cli.Context) error {
					mode := cCtx.Args().First()
					if mode == "debug" {
						progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
						progress.Suffix = " Preparing debug mode... "
						progress.Start()

						err := build("bima", true)
						if err != nil {
							progress.Stop()

							return err
						}

						progress.Stop()

						cmd, _ := syntax.NewParser().Parse(strings.NewReader(fmt.Sprintf("./bima run %s", file)), "")
						runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

						return runner.Run(context.TODO(), cmd)
					}

					return run(file)
				},
			},
			{
				Name:    "debug",
				Aliases: []string{"dbg"},
				Usage:   "debug ",
				Action: func(cCtx *cli.Context) error {
					argument := cCtx.Args().First()
					if argument == "" {
						fmt.Println("Usage: bima debug <pid>")

						return nil
					}

					pid, err := strconv.Atoi(argument)
					if err != nil {
						color.New(color.FgRed).Println("PID must a number")

						return nil
					}

					return debug(pid)
				},
			},
			{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "version",
				Action: func(*cli.Context) error {
					wd, _ := os.Getwd()
					var path strings.Builder

					path.WriteString(wd)
					path.WriteString("/go.mod")

					version := "unknown"
					mod, err := os.ReadFile(path.String())
					if err != nil {
						fmt.Printf("Framework: %s\n", version)
						fmt.Printf("Cli: %s\n", Version)

						return nil
					}

					f, err := modfile.Parse(path.String(), mod, nil)
					if err != nil {
						fmt.Printf("Framework: %s\n", version)
						fmt.Printf("Cli: %s\n", Version)

						return nil
					}

					for _, v := range f.Require {
						if v.Mod.Path == "github.com/bimalabs/framework/v4" {
							version = v.Mod.Version

							break
						}
					}

					fmt.Printf("Framework: %s\n", version)
					fmt.Printf("Cli: %s\n", Version)

					return nil
				},
			},
			{
				Name:    "upgrade",
				Aliases: []string{"upg"},
				Usage:   "upgrade",
				Action: func(*cli.Context) error {
					return upgrade()
				},
			},
			{
				Name:    "makesure",
				Aliases: []string{"mks"},
				Usage:   "makesure",
				Action: func(ctx *cli.Context) error {
					progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Checking toolchain installment... "
					progress.Start()

					if err := clean(); err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error cleaning dependencies")

						return err
					}

					progress.Stop()

					progress = spinner.New(spinner.CharSets[SpinerIndex], Duration)
					progress.Suffix = " Try to install/update to latest toolchain... "
					progress.Start()
					err := toolchain()
					if err != nil {
						progress.Stop()
						color.New(color.FgRed).Println("Error install toolchain")

						return err
					}

					progress.Stop()
					fmt.Println("Toolchain installed")

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func upgrade() error {
	temp := os.TempDir()
	os.RemoveAll(fmt.Sprintf("%s/bima", temp))

	progress := spinner.New(spinner.CharSets[SpinerIndex], Duration)
	progress.Suffix = " Checking new update... "
	progress.Start()

	wd := fmt.Sprintf("%sbima", temp)
	output, err := exec.Command("git", "clone", "--depth", "1", "https://github.com/bimalabs/cli.git", wd).CombinedOutput()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return nil
	}

	cmd := exec.Command("git", "rev-list", "--tags", "--max-count=1")
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()

	re := regexp.MustCompile(`\r?\n`)
	commitId := re.ReplaceAllString(string(output), "")

	cmd = exec.Command("git", "describe", "--tags", commitId)
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()

	re = regexp.MustCompile(`\r?\n`)
	latest := re.ReplaceAllString(string(output), "")
	if latest == Version {
		progress.Stop()
		color.New(color.FgGreen).Println("Bima Cli is already up to date")

		return nil
	}

	progress.Stop()

	progress = spinner.New(spinner.CharSets[SpinerIndex], Duration)
	progress.Suffix = " Updating Bima Cli... "
	progress.Start()

	cmd = exec.Command("git", "fetch")
	cmd.Dir = wd
	err = cmd.Run()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return nil
	}

	cmd = exec.Command("git", "checkout", latest)
	cmd.Dir = wd
	err = cmd.Run()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return nil
	}

	cmd = exec.Command("go", "get")
	cmd.Dir = wd
	cmd.Run()

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = wd
	cmd.Run()

	cmd = exec.Command("go", "run", "dumper/main.go")
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return err
	}

	cmd = exec.Command("go", "get", "-u")
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return err
	}

	cmd = exec.Command("go", "build", "-o", "bima")
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return err
	}

	cmd = exec.Command("mv", "bima", "/usr/local/bin/bima")
	cmd.Dir = wd
	output, err = cmd.CombinedOutput()
	if err != nil {
		progress.Stop()
		color.New(color.FgRed).Println(string(output))

		return err
	}

	progress.Stop()
	color.New(color.FgGreen).Print("Bima Cli is upgraded to ")
	color.New(color.FgGreen, color.Bold).Println(latest)

	return nil
}

func create(name string) error {
	output, err := exec.Command("git", "clone", "--depth", "1", "https://github.com/bimalabs/skeleton.git", name).CombinedOutput()
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	output, err = exec.Command("rm", "-rf", fmt.Sprintf("%s/.git", name)).CombinedOutput()
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	f, err := os.Create(fmt.Sprintf("%s/.env", name))
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	hasher := sha256.New()
	hasher.Write([]byte(time.Now().Format(time.RFC3339)))

	_, err = f.WriteString(fmt.Sprintf(Env, name, base64.URLEncoding.EncodeToString(hasher.Sum(nil))))
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	f.Sync()
	f.Close()

	wd, _ := os.Getwd()

	cmd := exec.Command("go", "get")
	cmd.Dir = fmt.Sprintf("%s/%s", wd, name)
	cmd.Run()

	cmd = exec.Command("go", "run", "dumper/main.go")
	cmd.Dir = fmt.Sprintf("%s/%s", wd, name)
	output, err = cmd.CombinedOutput()
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	cmd = exec.Command("go", "get", "-u")
	cmd.Dir = fmt.Sprintf("%s/%s", wd, name)
	output, err = cmd.CombinedOutput()
	if err != nil {
		color.New(color.FgRed).Println(string(output))

		return err
	}

	return nil
}

func debug(pid int) error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader(fmt.Sprintf("dlv attach %d --listen=:16517 --headless --api-version=2 --log", pid)), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func build(name string, debug bool) error {
	if debug {
		cmd, _ := syntax.NewParser().Parse(strings.NewReader(fmt.Sprintf("go build -gcflags \"all=-N -l\" -o %s cmd/main.go", name)), "")
		runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

		return runner.Run(context.TODO(), cmd)
	}

	cmd, _ := syntax.NewParser().Parse(strings.NewReader(fmt.Sprintf("go build -o %s cmd/main.go", name)), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func dump() error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader("go run dumper/main.go"), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func clean() error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader("go mod tidy"), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func toolchain() error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader(`go install \
github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
google.golang.org/protobuf/cmd/protoc-gen-go \
google.golang.org/grpc/cmd/protoc-gen-go-grpc
`), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func update() error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader("go get -u"), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func run(file string) error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader(fmt.Sprintf("go run cmd/main.go run %s", file)), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func genproto() error {
	cmd, _ := syntax.NewParser().Parse(strings.NewReader("sh proto_gen.sh"), "")
	runner, _ := interp.New(interp.Env(nil), interp.StdIO(nil, os.Stdout, os.Stdout))

	return runner.Run(context.TODO(), cmd)
}

func env(config *configs.Env, filePath string, ext string) {
	switch ext {
	case ".env":
		godotenv.Load()
		parse(config)
	case ".yaml":
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalln(err.Error())
		}

		err = yaml.Unmarshal(content, config)
		if err != nil {
			log.Fatalln(err.Error())
		}
	case ".json":
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalln(err.Error())
		}

		err = json.Unmarshal(content, config)
		if err != nil {
			log.Fatalln(err.Error())
		}
	}
}

func parse(config *configs.Env) {
	config.Secret = os.Getenv("APP_SECRET")
	config.Debug, _ = strconv.ParseBool(os.Getenv("APP_DEBUG"))
	config.HttpPort, _ = strconv.Atoi(os.Getenv("APP_PORT"))
	config.RpcPort, _ = strconv.Atoi(os.Getenv("GRPC_PORT"))
	config.Service = os.Getenv("APP_NAME")

	dbPort, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	config.Db = configs.Db{
		Host:     os.Getenv("DB_HOST"),
		Port:     dbPort,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		Driver:   os.Getenv("DB_DRIVER"),
	}

	config.CacheLifetime, _ = strconv.Atoi(os.Getenv("CACHE_LIFETIME"))
}

func register(generator *generators.Factory, util *color.Color, name string) error {
	module := generators.ModuleTemplate{}
	field := generators.FieldTemplate{}
	mapType := utils.NewType()

	util.Println("Welcome to Bima Skeleton Module Generator")
	module.Name = name

	index := 2
	more := true
	for more {
		err := interact.NewInteraction("Add new column?").Resolve(&more)
		if err != nil {
			color.New(color.FgRed).Println(err.Error())

			return err
		}

		if more {
			column(util, &field, mapType)

			field.Name = strings.Replace(field.Name, " ", "", -1)
			column := generators.FieldTemplate{}

			copier.Copy(&column, field)

			column.Index = index
			column.Name = strings.Title(column.Name)
			column.NameUnderScore = strcase.ToDelimited(column.Name, '_')
			module.Fields = append(module.Fields, &column)

			field.Name = ""
			field.ProtobufType = ""

			index++
		}
	}

	if len(module.Fields) < 1 {
		return errors.New("You must have at least one column in table")
	}

	generator.Generate(module)

	workDir, _ := os.Getwd()
	fmt.Print("Module ")
	util.Print(name)
	fmt.Printf(" registered in %s/modules.yaml\n", workDir)

	return nil
}

func unregister(util *color.Color, module string) {
	workDir, _ := os.Getwd()
	pluralizer := pluralize.NewClient()
	moduleName := strcase.ToCamel(pluralizer.Singular(module))
	modulePlural := strcase.ToDelimited(pluralizer.Plural(moduleName), '_')
	moduleUnderscore := strcase.ToDelimited(module, '_')
	list := parsers.ParseModule(workDir)

	exist := false
	for _, v := range list {
		if v == fmt.Sprintf("module:%s", moduleUnderscore) {
			exist = true
			break
		}
	}

	if !exist {
		util.Println("Module is not registered")
		return
	}

	mod, err := os.ReadFile(fmt.Sprintf("%s/go.mod", workDir))
	if err != nil {
		panic(err)
	}

	jsonModules := fmt.Sprintf("%s/swaggers/modules.json", workDir)
	file, _ := os.ReadFile(jsonModules)
	modulesJson := []generators.ModuleJson{}
	registered := modulesJson
	json.Unmarshal(file, &modulesJson)
	for _, v := range modulesJson {
		if v.Name != moduleName {
			mUrl, _ := url.Parse(v.Url)
			query := mUrl.Query()

			query.Set("v", strconv.Itoa(int(time.Now().UnixMicro())))
			mUrl.RawQuery = query.Encode()
			v.Url = mUrl.String()
			registered = append(registered, v)
		}
	}

	registeredByte, _ := json.Marshal(registered)
	os.WriteFile(jsonModules, registeredByte, 0644)

	packageName := modfile.ModulePath(mod)
	yaml := fmt.Sprintf("%s/configs/modules.yaml", workDir)
	file, _ = os.ReadFile(yaml)
	modules := string(file)

	provider := fmt.Sprintf("%s/configs/provider.go", workDir)
	file, _ = os.ReadFile(provider)
	codeblock := string(file)

	modRegex := regexp.MustCompile(fmt.Sprintf("(?m)[\r\n]+^.*module:%s.*$", moduleUnderscore))
	modules = modRegex.ReplaceAllString(modules, "")
	os.WriteFile(yaml, []byte(modules), 0644)

	regex := regexp.MustCompile(fmt.Sprintf("(?m)[\r\n]+^.*%s.*$", fmt.Sprintf("%s/%s", packageName, modulePlural)))
	codeblock = regex.ReplaceAllString(codeblock, "")

	codeblock = modRegex.ReplaceAllString(codeblock, "")
	os.WriteFile(provider, []byte(codeblock), 0644)

	os.RemoveAll(fmt.Sprintf("%s/%s", workDir, modulePlural))
	os.Remove(fmt.Sprintf("%s/protos/%s.proto", workDir, moduleUnderscore))
	os.Remove(fmt.Sprintf("%s/protos/builds/%s_grpc.pb.go", workDir, moduleUnderscore))
	os.Remove(fmt.Sprintf("%s/protos/builds/%s.pb.go", workDir, moduleUnderscore))
	os.Remove(fmt.Sprintf("%s/protos/builds/%s.pb.gw.go", workDir, moduleUnderscore))
	os.Remove(fmt.Sprintf("%s/swaggers/%s.swagger.json", workDir, moduleUnderscore))

	fmt.Print("Module ")
	util.Print(module)
	util.Println(" deleted")
}

func column(util *color.Color, field *generators.FieldTemplate, mapType utils.Type) {
	err := interact.NewInteraction("Input column name?").Resolve(&field.Name)
	if err != nil {
		util.Println(err.Error())
		column(util, field, mapType)
	}

	if field.Name == "" {
		util.Println("Column name is required")
		column(util, field, mapType)
	}

	field.ProtobufType = "string"
	err = interact.NewInteraction("Input data type?",
		interact.Choice{Display: "string", Value: "string"},
		interact.Choice{Display: "bool", Value: "bool"},
		interact.Choice{Display: "int32", Value: "int32"},
		interact.Choice{Display: "int64", Value: "int64"},
		interact.Choice{Display: "bytes", Value: "bytes"},
		interact.Choice{Display: "double", Value: "double"},
		interact.Choice{Display: "float", Value: "float"},
		interact.Choice{Display: "uint32", Value: "uint32"},
		interact.Choice{Display: "sint32", Value: "sint32"},
		interact.Choice{Display: "sint64", Value: "sint64"},
		interact.Choice{Display: "fixed32", Value: "fixed32"},
		interact.Choice{Display: "fixed64", Value: "fixed64"},
		interact.Choice{Display: "sfixed32", Value: "sfixed32"},
		interact.Choice{Display: "sfixed64", Value: "sfixed64"},
	).Resolve(&field.ProtobufType)
	if err != nil {
		util.Println(err.Error())
		column(util, field, mapType)
	}

	field.GolangType = mapType.Value(field.ProtobufType)
	field.IsRequired = true
	err = interact.NewInteraction("Is column required?").Resolve(&field.IsRequired)
	if err != nil {
		util.Println(err.Error())
		column(util, field, mapType)
	}
}
