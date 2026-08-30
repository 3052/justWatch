package main

import (
   "encoding/json"
   "flag"
   "fmt"
   "log"
   "os"
   "sort"
   "strings"
)

func main() {
   fileFlag := flag.String("file", "", "Path to JSON file containing an array of objects (required)")
   flag.Parse()

   if *fileFlag == "" {
      flag.Usage()
      log.Fatal("Error: the -file flag is required.")
   }

   fileBytes, err := os.ReadFile(*fileFlag)
   if err != nil {
      log.Fatalf("Failed to read file '%s': %v", *fileFlag, err)
   }

   var inputs []ProviderInput
   if err := json.Unmarshal(fileBytes, &inputs); err != nil {
      log.Fatalf("Failed to parse JSON data: %v", err)
   }

   var results []Result

   for i, input := range inputs {
      path := strings.TrimSpace(input.Path)
      if path == "" {
         continue
      }
      date := strings.TrimSpace(input.Date)

      // Progress indicator (writes to stderr)
      log.Printf("[%d/%d] Processing: %s...", i+1, len(inputs), path)

      // 1. Get Locale from Path
      locale, err := fetchLocaleFromPath(path)
      if err != nil {
         log.Fatalf("[%s] Failed to get metadata: %v\n", path, err)
      }

      parts := strings.Split(locale, "_")
      if len(parts) != 2 {
         log.Fatalf("[%s] Invalid locale format received: %s\n", path, locale)
      }
      // Extract country directly from the path to match user examples (e.g. /uk/ -> UK)
      pathSegments := strings.Split(strings.Trim(path, "/"), "/")
      displayCountry := strings.ToUpper(pathSegments[0])

      // 2. Resolve the Package Code & Clear Name using the Locale & Path
      pkgCode, clearName, err := resolvePackageFromPath(path, locale)
      if err != nil {
         log.Fatalf("[%s] Failed to resolve package code: %v\n", path, err)
      }

      // 3. Fetch the Total Count (using the API's actual country code from locale, e.g. US, GB, CZ)
      apiCountry := parts[1]
      totalCount, err := fetchTotalCount(pkgCode, apiCountry)
      if err != nil {
         log.Fatalf("[%s] Failed to fetch total count: %v\n", path, err)
      }

      // Show completion for this item (writes to stderr)
      log.Printf("[%d/%d] Done: [%s] %s -> %d titles", i+1, len(inputs), displayCountry, clearName, totalCount)

      results = append(results, Result{
         Count:     totalCount,
         Country:   displayCountry,
         ClearName: clearName,
         Path:      path,
         Date:      date,
      })
   }

   // --- First Table: Sorted by Title Count DESC ---
   sort.Slice(results, func(i, j int) bool {
      return results[i].Count > results[j].Count
   })

   fmt.Println("\n| Titles | Country | Provider |")
   fmt.Println("|---|---|---|")
   for _, r := range results {
      fmt.Printf("| %d | %s | [%s] |\n", r.Count, r.Country, r.ClearName)
   }

   // --- Second Table: Sorted by Date DESC with chronological Yearly Count ---

   // 1. Sort ASC by Date to assign chronological counts (oldest = 1)
   sort.Slice(results, func(i, j int) bool {
      return results[i].Date < results[j].Date
   })

   var currentYear string
   var yearCount int
   for i := range results {
      year := ""
      if len(results[i].Date) >= 4 {
         year = results[i].Date[:4]
      }

      if year != currentYear {
         currentYear = year
         yearCount = 1
      } else {
         yearCount++
      }
      results[i].YearRank = yearCount
   }

   // 2. Sort DESC by Date for display
   sort.Slice(results, func(i, j int) bool {
      return results[i].Date > results[j].Date
   })

   fmt.Println("\n| Date | Country | Provider | Count |")
   fmt.Println("|---|---|---|---|")

   for _, r := range results {
      // Printed without Markdown link brackets for the provider
      fmt.Printf("| %s | %s | %s | %d |\n", r.Date, r.Country, r.ClearName, r.YearRank)
   }

   fmt.Println()

   // Print Markdown Links (writes to stdout)
   // Used only by the first table
   for _, r := range results {
      fmt.Printf("[%s]:https://justwatch.com%s?tomatoMeter=%d\n", r.ClearName, r.Path, TomatoMeterMin)
   }
}

// Struct to read the updated JSON format
type ProviderInput struct {
   Path string `json:"path"`
   Date string `json:"date"`
}

// Result models the final data for our Markdown tables
type Result struct {
   Count     int
   Country   string
   ClearName string
   Path      string
   Date      string
   YearRank  int
}
