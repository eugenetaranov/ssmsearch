package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"ssmsearch/internal/cache"
	"ssmsearch/internal/ssm"
)

var (
	version   = "dev"
	gitCommit = "unknown"
)

func main() {
	// Simple CLI flags
	listAll := flag.Bool("l", false, "list all SSM parameters")
	search := flag.Bool("s", false, "search mode - terms as positional args")
	refresh := flag.Bool("r", false, "refresh cache (use with -s)")
	tree := flag.Bool("t", false, "display output as tree")
	copyParam := flag.Bool("cp", false, "copy parameter: -cp src dest1 [dest2 ...]")
	writeParam := flag.Bool("w", false, "write parameter: -w /path [value]")
	yes := flag.Bool("y", false, "skip confirmation prompt (use with -w or -cp)")
	showVersion := flag.Bool("v", false, "show version and exit")

	// AWS options
	region := flag.String("region", "", "AWS region")
	profile := flag.String("profile", "", "AWS profile")
	path := flag.String("p", "/", "parameter path prefix for listing")
	decrypt := flag.Bool("d", true, "decrypt SecureString values")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ssmsearch - AWS SSM Parameter Store CLI\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -l              List all parameters\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -l -p /app/     List parameters under /app/\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -s term1 term2  Search parameters matching all terms\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -s -r term      Refresh cache and search\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -r              Refresh cache only\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -t              Tree view of all parameters\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -t -p /app/     Tree view under /app/\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -t -s term      Tree view of search results\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -cp src dest    Copy parameter value from src to dest\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -w /path value  Write/update parameter value\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -w /path        Prompt for value interactively\n")
		fmt.Fprintf(os.Stderr, "  echo val | ssmsearch -w /path  Write value from stdin\n")
		fmt.Fprintf(os.Stderr, "  ssmsearch -v              Show version\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
	}

	os.Args = reorderArgs(os.Args)
	flag.Parse()

	// Version
	if *showVersion {
		fmt.Printf("ssmsearch %s (commit: %s)\n", version, gitCommit)
		os.Exit(0)
	}

	// Collect search terms from positional args
	searchTerms := flag.Args()

	// Validate search mode requires terms
	if *search && len(searchTerms) == 0 {
		fmt.Fprintf(os.Stderr, "Error: -s requires at least one search term\n")
		os.Exit(1)
	}

	// Validate copy mode requires at least 2 args and no other flags
	if *copyParam {
		if len(searchTerms) < 2 {
			fmt.Fprintf(os.Stderr, "Error: -cp requires source and at least one destination\n")
			os.Exit(1)
		}
		if *listAll || *search || *refresh || *tree || *writeParam {
			fmt.Fprintf(os.Stderr, "Error: -cp cannot be used with other flags\n")
			os.Exit(1)
		}
	}

	// Validate write mode requires one or two args and no other flags
	if *writeParam {
		if len(searchTerms) < 1 || len(searchTerms) > 2 {
			fmt.Fprintf(os.Stderr, "Error: -w requires parameter path and optional value\n")
			os.Exit(1)
		}
		if !strings.HasPrefix(searchTerms[0], "/") {
			fmt.Fprintf(os.Stderr, "Error: parameter path must start with /\n")
			os.Exit(1)
		}
		if *listAll || *search || *refresh || *tree || *copyParam {
			fmt.Fprintf(os.Stderr, "Error: -w cannot be used with other flags\n")
			os.Exit(1)
		}
	}

	// Show help if no command specified
	if !*listAll && !*search && !*refresh && !*tree && !*copyParam && !*writeParam {
		flag.Usage()
		os.Exit(0)
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create SSM client
	client, err := ssm.NewClient(ctx, ssm.ClientOptions{
		Region:  *region,
		Profile: *profile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var params []ssm.Parameter

	switch {
	case *writeParam:
		var valueArg string
		if len(searchTerms) == 2 {
			valueArg = searchTerms[1]
		}
		err = writeParameter(ctx, client, searchTerms[0], valueArg, *yes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case *copyParam:
		err = copyParameter(ctx, client, searchTerms[0], searchTerms[1:], *yes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case *tree && !*search:
		params, err = client.ListParameters(ctx, ssm.ListOptions{
			Path:      *path,
			Recursive: true,
			Decrypt:   false, // tree only shows names
		})
	case *listAll && *search:
		params, err = searchKeysOnly(ctx, client, searchTerms, *path, *refresh)
	case *listAll:
		params, err = client.ListParameters(ctx, ssm.ListOptions{
			Path:      *path,
			Recursive: true,
			Decrypt:   false, // list only shows names
		})
	case *tree && *search:
		params, err = searchKeysOnly(ctx, client, searchTerms, *path, *refresh)
	case *search:
		params, err = searchWithCache(ctx, client, searchTerms, *path, *refresh, *decrypt)
	case *refresh:
		err = refreshCache(ctx, client)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output params
	if *tree {
		printTree(params, *path)
	} else if *listAll {
		for _, p := range params {
			fmt.Println(p.Name)
		}
	} else {
		for _, p := range params {
			fmt.Printf("%s %s\n", p.Name, p.Value)
		}
	}
}

type treeNode struct {
	children map[string]*treeNode
}

func buildTree(params []ssm.Parameter, prefix string) *treeNode {
	root := &treeNode{children: make(map[string]*treeNode)}

	for _, p := range params {
		// Strip prefix for cleaner display
		path := strings.TrimPrefix(p.Name, prefix)
		path = strings.TrimPrefix(path, "/")
		parts := strings.Split(path, "/")

		node := root
		for _, part := range parts {
			if node.children[part] == nil {
				node.children[part] = &treeNode{
					children: make(map[string]*treeNode),
				}
			}
			node = node.children[part]
		}
	}
	return root
}

func printTree(params []ssm.Parameter, prefix string) {
	if len(params) == 0 {
		return
	}

	root := buildTree(params, prefix)

	// Print prefix as root
	if prefix != "/" {
		fmt.Println(prefix)
	} else {
		fmt.Println("/")
	}

	printNode(root, "")
}

func printNode(node *treeNode, indent string) {
	// Get sorted children names
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		child := node.children[name]
		isLast := i == len(names)-1

		// Print branch
		if isLast {
			fmt.Print(indent + "└── ")
		} else {
			fmt.Print(indent + "├── ")
		}

		// Print name
		fmt.Println(name)

		// Recurse with updated indent
		newIndent := indent
		if isLast {
			newIndent += "    "
		} else {
			newIndent += "│   "
		}
		printNode(child, newIndent)
	}
}

func copyParameter(ctx context.Context, client ssm.Client, src string, dests []string, skipConfirm bool) error {
	// Get source parameter
	params, err := client.GetParameters(ctx, []string{src}, true)
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return fmt.Errorf("source parameter not found: %s", src)
	}

	srcParam := params[0]
	var reader *bufio.Reader
	if !skipConfirm {
		reader = bufio.NewReader(os.Stdin)
	}

	for _, dest := range dests {
		if !skipConfirm {
			fmt.Printf("Copy %s -> %s? [y/N] ", src, dest)
			answer, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Printf("Skipped %s\n", dest)
				continue
			}
		}

		err = client.PutParameter(ctx, dest, srcParam.Value, srcParam.Type, true)
		if err != nil {
			return err
		}
		fmt.Printf("Copied %s -> %s\n", src, dest)
	}

	return nil
}

func writeParameter(ctx context.Context, client ssm.Client, path, valueArg string, skipConfirm bool) error {
	// Check if parameter exists to preserve type
	paramType := "String"
	existing, err := client.GetParameters(ctx, []string{path}, false)
	if err == nil && len(existing) > 0 {
		paramType = existing[0].Type
	}

	var valueStr string

	if valueArg != "" {
		// Use value from argument
		valueStr = valueArg
	} else {
		// Check if stdin is piped
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Read value from stdin pipe
			value, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			valueStr = string(value)
		} else {
			// Interactive: prompt for value
			tty, err := os.Open("/dev/tty")
			if err != nil {
				return fmt.Errorf("cannot open terminal: %w", err)
			}
			defer tty.Close()

			fmt.Printf("Enter value for %s (%s) — press Ctrl+D when done:\n", path, paramType)
			value, err := io.ReadAll(tty)
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			valueStr = string(value)
		}
	}

	valueStr = strings.TrimSpace(valueStr)
	if valueStr == "" {
		return fmt.Errorf("empty value provided")
	}

	if !skipConfirm {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("cannot open terminal for confirmation: %w", err)
		}
		defer tty.Close()

		fmt.Printf("Write to %s (%s)? [y/N] ", path, paramType)
		reader := bufio.NewReader(tty)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	err = client.PutParameter(ctx, path, valueStr, paramType, true)
	if err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}

func refreshCache(ctx context.Context, client ssm.Client) error {
	accountID, err := client.GetAccountID(ctx)
	if err != nil {
		return err
	}

	c, err := cache.New()
	if err != nil {
		return err
	}

	// Remove existing cache
	if err := c.Remove(accountID); err != nil {
		return err
	}

	// Fetch all parameter names from AWS
	allParams, err := client.ListParameters(ctx, ssm.ListOptions{
		Path:      "/",
		Recursive: true,
		Decrypt:   false,
	})
	if err != nil {
		return err
	}

	keys := make([]string, len(allParams))
	for i, p := range allParams {
		keys[i] = p.Name
	}

	// Save to cache
	if err := c.Save(accountID, keys); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Cache refreshed: %d parameters\n", len(keys))
	return nil
}

func searchWithCache(ctx context.Context, client ssm.Client, terms []string, pathPrefix string, refresh, decrypt bool) ([]ssm.Parameter, error) {
	// Get account ID for cache file
	accountID, err := client.GetAccountID(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize cache
	c, err := cache.New()
	if err != nil {
		return nil, err
	}

	var keys []string

	// Refresh: delete existing cache
	if refresh {
		if err := c.Remove(accountID); err != nil {
			return nil, err
		}
	}

	// Load from cache or fetch from AWS
	if !refresh && c.Exists(accountID) {
		keys, err = c.Load(accountID)
		if err != nil {
			return nil, err
		}
	} else {
		// Fetch all parameter names from AWS
		allParams, err := client.ListParameters(ctx, ssm.ListOptions{
			Path:      "/",
			Recursive: true,
			Decrypt:   false, // Don't decrypt, we only need names
		})
		if err != nil {
			return nil, err
		}

		keys = make([]string, len(allParams))
		for i, p := range allParams {
			keys[i] = p.Name
		}

		// Save to cache
		if err := c.Save(accountID, keys); err != nil {
			return nil, err
		}
	}

	// Filter by path prefix
	if pathPrefix != "/" {
		var prefixedKeys []string
		for _, k := range keys {
			if strings.HasPrefix(k, pathPrefix) {
				prefixedKeys = append(prefixedKeys, k)
			}
		}
		keys = prefixedKeys
	}

	// Filter keys by search terms
	matchedKeys := fuzzyFilterKeys(keys, terms)

	if len(matchedKeys) == 0 {
		return nil, nil
	}

	// Fetch values for matched keys
	return client.GetParameters(ctx, matchedKeys, decrypt)
}

func searchKeysOnly(ctx context.Context, client ssm.Client, terms []string, pathPrefix string, refresh bool) ([]ssm.Parameter, error) {
	// Get account ID for cache file
	accountID, err := client.GetAccountID(ctx)
	if err != nil {
		return nil, err
	}

	// Initialize cache
	c, err := cache.New()
	if err != nil {
		return nil, err
	}

	var keys []string

	// Refresh: delete existing cache
	if refresh {
		if err := c.Remove(accountID); err != nil {
			return nil, err
		}
	}

	// Load from cache or fetch from AWS
	if !refresh && c.Exists(accountID) {
		keys, err = c.Load(accountID)
		if err != nil {
			return nil, err
		}
	} else {
		// Fetch all parameter names from AWS
		allParams, err := client.ListParameters(ctx, ssm.ListOptions{
			Path:      "/",
			Recursive: true,
			Decrypt:   false,
		})
		if err != nil {
			return nil, err
		}

		keys = make([]string, len(allParams))
		for i, p := range allParams {
			keys[i] = p.Name
		}

		// Save to cache
		if err := c.Save(accountID, keys); err != nil {
			return nil, err
		}
	}

	// Filter by path prefix
	if pathPrefix != "/" {
		var prefixedKeys []string
		for _, k := range keys {
			if strings.HasPrefix(k, pathPrefix) {
				prefixedKeys = append(prefixedKeys, k)
			}
		}
		keys = prefixedKeys
	}

	// Filter keys by search terms
	matchedKeys := fuzzyFilterKeys(keys, terms)

	// Return as Parameters with only Name set (no value fetch)
	params := make([]ssm.Parameter, len(matchedKeys))
	for i, k := range matchedKeys {
		params[i] = ssm.Parameter{Name: k}
	}
	return params, nil
}

// fuzzyFilterKeys filters parameter keys by fuzzy matching.
// All patterns must match (AND logic).
func fuzzyFilterKeys(keys []string, patterns []string) []string {
	var result []string

	for _, key := range keys {
		name := strings.ToLower(key)

		allMatch := true
		for _, pattern := range patterns {
			pattern = strings.ToLower(pattern)
			if !fuzzyMatch(name, pattern) {
				allMatch = false
				break
			}
		}
		if allMatch {
			result = append(result, key)
		}
	}

	return result
}

// fuzzyMatch checks if text contains the pattern as a substring.
func fuzzyMatch(text, pattern string) bool {
	return pattern == "" || strings.Contains(text, pattern)
}

// reorderArgs moves flags before positional arguments to work around
// Go's flag package stopping at the first non-flag argument.
func reorderArgs(args []string) []string {
	var flags, positional []string
	flags = append(flags, args[0]) // program name

	i := 1
	for i < len(args) {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			// Check if this flag takes a value: -p, -region, -profile
			if i+1 < len(args) && (args[i] == "-p" || args[i] == "-region" || args[i] == "-profile") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, args[i])
		}
		i++
	}
	return append(flags, positional...)
}
