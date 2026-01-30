package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
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

	// Show help if no command specified
	if !*listAll && !*search && !*refresh {
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
	case *listAll:
		params, err = client.ListParameters(ctx, ssm.ListOptions{
			Path:      *path,
			Recursive: true,
			Decrypt:   *decrypt,
		})
	case *search:
		params, err = searchWithCache(ctx, client, searchTerms, *path, *refresh, *decrypt)
	case *refresh:
		err = refreshCache(ctx, client)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Output params, one per line: name value
	for _, p := range params {
		fmt.Printf("%s %s\n", p.Name, p.Value)
	}
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

// fuzzyFilter filters parameters by fuzzy matching on name or value.
// All patterns must match (AND logic).
func fuzzyFilter(params []ssm.Parameter, patterns []string) []ssm.Parameter {
	var result []ssm.Parameter

	for _, p := range params {
		name := strings.ToLower(p.Name)
		value := strings.ToLower(p.Value)

		allMatch := true
		for _, pattern := range patterns {
			pattern = strings.ToLower(pattern)
			if !fuzzyMatch(name, pattern) && !fuzzyMatch(value, pattern) {
				allMatch = false
				break
			}
		}
		if allMatch {
			result = append(result, p)
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
