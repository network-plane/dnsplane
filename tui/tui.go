package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Command represents an executable command in a context.
type Command interface {
	Name() string
	Help() string
	Exec(args []string)
}

// context holds commands for a specific context.
type context struct {
	name        string
	description string
	commands    map[string]Command
}

var (
	contexts     = map[string]*context{"": {name: "", commands: map[string]Command{}}}
	currentCtx   = ""
	basePrompt   = "> "
	helpHeader   = "Available commands:"
	excludedCmds = map[string]bool{"q": true, "quit": true, "exit": true, "h": true, "help": true, "ls": true, "l": true, "/": true}
)

// SetPrompt sets the base prompt prefix.
func SetPrompt(p string) { basePrompt = p }

// SetHelpHeader allows customising the help text header.
func SetHelpHeader(h string) { helpHeader = h }

// RegisterContext registers a new context with an optional description.
func RegisterContext(name, description string) {
	if _, ok := contexts[name]; !ok {
		contexts[name] = &context{name: name, description: description, commands: map[string]Command{}}
	} else {
		contexts[name].description = description
	}
}

// RegisterCommand registers a command under the specified context.
func RegisterCommand(ctxName string, cmd Command) {
	ctx, ok := contexts[ctxName]
	if !ok {
		ctx = &context{name: ctxName, commands: map[string]Command{}}
		contexts[ctxName] = ctx
	}
	ctx.commands[cmd.Name()] = cmd
}

// Run starts the main input loop using the provided readline instance.
func Run(rl *readline.Instance) {
	setupAutocomplete(rl)
	for {
		updatePrompt(rl)
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}
		if isExitCommand(args[0]) {
			fmt.Println("Shutting down.")
			os.Exit(0)
		}
		if !excludedCmds[args[0]] {
			if err := rl.SaveHistory(line); err != nil {
				fmt.Println("Error saving history:", err)
			}
		}
		if currentCtx == "" {
			handleRootCommand(args, rl)
		} else {
			handleContextCommand(args, rl)
		}
	}
}

func isExitCommand(cmd string) bool {
	return cmd == "q" || cmd == "quit" || cmd == "exit"
}

func handleRootCommand(args []string, rl *readline.Instance) {
	switch args[0] {
	case "clear":
		fmt.Print("\033[H\033[2J")
		rl.Refresh()
	case "help", "h", "?", "ls", "l":
		showHelp("")
	default:
		if ctx, ok := contexts[args[0]]; ok {
			if len(args) == 1 {
				currentCtx = ctx.name
				setupAutocomplete(rl)
				return
			}
			if cmd, ok := ctx.commands[args[1]]; ok {
				cmd.Exec(args[2:])
			} else {
				fmt.Printf("Unknown command: %s\n", args[1])
			}
		} else if cmd, ok := contexts[""].commands[args[0]]; ok {
			cmd.Exec(args[1:])
		} else {
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", args[0])
		}
	}
}

func handleContextCommand(args []string, rl *readline.Instance) {
	if args[0] == "/" {
		currentCtx = ""
		setupAutocomplete(rl)
		return
	}
	if args[0] == "help" || args[0] == "?" || args[0] == "h" {
		showHelp(currentCtx)
		return
	}
	ctx := contexts[currentCtx]
	if cmd, ok := ctx.commands[args[0]]; ok {
		cmd.Exec(args[1:])
	} else {
		fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", args[0])
	}
}

func updatePrompt(rl *readline.Instance) {
	if currentCtx == "" {
		rl.SetPrompt(basePrompt)
	} else {
		title := cases.Title(language.English).String(currentCtx)
		rl.SetPrompt(fmt.Sprintf("%s%s> ", basePrompt, title))
	}
}

func setupAutocomplete(rl *readline.Instance) {
	if currentCtx == "" {
		var items []readline.PrefixCompleterInterface
		for name := range contexts {
			if name == "" {
				continue
			}
			items = append(items, readline.PcItem(name))
		}
		for name := range contexts[""].commands {
			items = append(items, readline.PcItem(name))
		}
		rl.Config.AutoComplete = readline.NewPrefixCompleter(items...)
	} else {
		ctx := contexts[currentCtx]
		var cmds []string
		for name := range ctx.commands {
			cmds = append(cmds, name)
		}
		rl.Config.AutoComplete = readline.NewPrefixCompleter(
			readline.PcItemDynamic(func(string) []string { return cmds }),
		)
	}
}

func showHelp(ctxName string) {
	fmt.Println(helpHeader)
	if ctxName == "" {
		keys := make([]string, 0, len(contexts))
		for name := range contexts {
			if name == "" {
				continue
			}
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			ctx := contexts[name]
			fmt.Printf("%-15s %s\n", name, ctx.description)
		}
		rootCmds := contexts[""]
		if len(rootCmds.commands) > 0 {
			fmt.Println()
			fmt.Println("Global Commands:")
			printCommands(rootCmds.commands, true)
		}
		commonHelp(false)
	} else {
		ctx := contexts[ctxName]
		printCommands(ctx.commands, false)
		commonHelp(true)
	}
}

func printCommands(cmds map[string]Command, indent bool) {
	keys := make([]string, 0, len(cmds))
	for name := range cmds {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	indentation := ""
	if indent {
		indentation = "  "
	}
	for _, name := range keys {
		fmt.Printf("%s%-15s %s\n", indentation, name, cmds[name].Help())
	}
}

func commonHelp(indent bool) {
	indentation := ""
	if indent {
		indentation = "  "
	}
	fmt.Printf("%s%-15s %s\n", indentation, "/", "- Go up one level")
	fmt.Printf("%s%-15s %s\n", indentation, "exit, quit, q", "- Shutdown the server")
	fmt.Printf("%s%-15s %s\n", indentation, "help, h, ?", "- Show help")
}
