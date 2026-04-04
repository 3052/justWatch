package main

import (
   "encoding/json"
   "flag"
   "fmt"
   "log"
   "net/http"
   "net/url"
   "os"
   "sort"
   "strings"
   "time"
)

func main() {
   // 1. Define and parse the command-line flags
   inputFile := flag.String("input", "", "Path to the JSON file containing URLs (required)")
   yearFlag := flag.Int("year", 0, "Release year from (e.g., 2025) (required)")
   flag.Parse()

   if *inputFile == "" || *yearFlag == 0 {
      fmt.Println("Error: both -input and -year flags are required.")
      flag.Usage()
      os.Exit(1)
   }

   // 2. Read the JSON file containing the URLs
   fileData, err := os.ReadFile(*inputFile)
   if err != nil {
      log.Fatalf("Failed to read file '%s': %v", *inputFile, err)
   }

   var urls []string
   if err := json.Unmarshal(fileData, &urls); err != nil {
      log.Fatalf("Failed to parse JSON in '%s': %v", *inputFile, err)
   }

   // 3. Process URLs sequentially (one at a time)
   var results []Result

   // Custom HTTP client with a timeout
   client := &http.Client{Timeout: 15 * time.Second}

   fmt.Println("Fetching data sequentially, please wait...")

   for i, rawTargetURL := range urls {
      // Safely parse the URL and add the query string parameter
      u, err := url.Parse(rawTargetURL)
      if err != nil {
         log.Printf("[%d/%d] Skipping invalid URL %s: %v\n", i+1, len(urls), rawTargetURL, err)
         continue
      }

      q := u.Query()
      q.Set("release_year_from", fmt.Sprintf("%d", *yearFlag))
      u.RawQuery = q.Encode()
      targetURL := u.String()

      fmt.Printf("[%d/%d] Fetching %s...\n", i+1, len(urls), targetURL)

      // Pass the year flag into processURL
      res, err := processURL(client, targetURL, *yearFlag)
      if err != nil {
         // If it's the specific totalCount error, halt the entire program immediately
         if strings.Contains(err.Error(), "totalCount not found in JSON") {
            log.Fatalf("  -> Fatal Error: %v. Stopping immediately.", err)
         }

         // For other errors (like timeouts), just log and continue
         log.Printf("  -> Error: %v\n", err)
         continue
      }

      results = append(results, res)
   }

   // 4. Sort the results descending by TotalCount
   sort.Slice(results, func(i, j int) bool {
      return results[i].TotalCount > results[j].TotalCount
   })

   // 5. Output as a Markdown table
   fmt.Printf("\n### Results (From Year %d)\n\n", *yearFlag)
   fmt.Println("| # | Country | Provider | Titles |")
   fmt.Println("|---|---|---|---|")
   for i, r := range results {
      fmt.Printf("| %d | %s | %s | %.0f |\n", i+1, r.Country, r.Provider, r.TotalCount)
   }
}
