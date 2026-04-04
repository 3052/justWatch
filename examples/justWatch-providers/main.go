package main

import (
   "encoding/json"
   "flag"
   "fmt"
   "log"
   "net/http"
   "os"
   "sort"
   "strings"
   "time"
)

func main() {
   // 1. Define and parse the command-line flag for the input file
   inputFile := flag.String("input", "", "Path to the JSON file containing URLs (required)")
   flag.Parse()

   if *inputFile == "" {
      fmt.Println("Error: the -input flag is required.")
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

   for i, targetURL := range urls {
      fmt.Printf("[%d/%d] Fetching %s...\n", i+1, len(urls), targetURL)

      res, err := processURL(client, targetURL)
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

   // 5. Output as an ordered list
   fmt.Println("\nResults:")
   for i, r := range results {
      // Wrapped the titles count in parentheses
      fmt.Printf("%d. %s %s (%.0f titles)\n", i+1, r.Country, r.Provider, r.TotalCount)
   }
}
