package main

import (
   "fmt"
   "io"
   "log"
   "os"
   "regexp"
   "sort"
)

// Provider holds the extracted properties
type Provider struct {
   Slug      string
   HasTitles string
}

func main() {
   // Assume the data is stored in a local file named "input.txt"
   // Change this to the actual path of your file.
   filename := "input.txt"
   file, err := os.Open(filename)
   if err != nil {
      log.Fatalf("Failed to open file: %v", err)
   }
   defer file.Close()

   content, err := io.ReadAll(file)
   if err != nil {
      log.Fatalf("Failed to read file: %v", err)
   }

   text := string(content)

   // Regex to match the minified JavaScript assignments. 
   // e.g., capturing the variable name (like 'a', 'b', 'cZ') and the value.
   slugRe := regexp.MustCompile(`\b([a-zA-Z0-9_$]+)\.slug="([^"]+)"`)
   hasTitlesRe := regexp.MustCompile(`\b([a-zA-Z0-9_$]+)\.hasTitles=(true|false)`)

   slugMatches := slugRe.FindAllStringSubmatch(text, -1)
   hasTitlesMatches := hasTitlesRe.FindAllStringSubmatch(text, -1)

   // Temporary struct to help merge properties belonging to the same JS variable
   type tempProviderData struct {
      Slug      string
      HasTitles string
      FoundSlug bool
      FoundHT   bool
   }

   providerMap := make(map[string]*tempProviderData)

   // Extract and map slugs
   for _, match := range slugMatches {
      varName := match[1]
      if providerMap[varName] == nil {
         providerMap[varName] = &tempProviderData{}
      }
      providerMap[varName].Slug = match[2]
      providerMap[varName].FoundSlug = true
   }

   // Extract and map hasTitles
   for _, match := range hasTitlesMatches {
      varName := match[1]
      if providerMap[varName] == nil {
         providerMap[varName] = &tempProviderData{}
      }
      providerMap[varName].HasTitles = match[2]
      providerMap[varName].FoundHT = true
   }

   // Collect valid providers (objects that have BOTH a slug and a hasTitles field)
   var providers[]Provider
   for _, data := range providerMap {
      if data.FoundSlug && data.FoundHT {
         providers = append(providers, Provider{
            Slug:      data.Slug,
            HasTitles: data.HasTitles,
         })
      }
   }

   // Sort alphabetically by Slug for clean, deterministic output
   sort.Slice(providers, func(i, j int) bool {
      return providers[i].Slug < providers[j].Slug
   })

   // Output the results
   fmt.Println("Extracted Providers:")
   fmt.Printf("%-45s | %s\n", "Slug", "HasTitles")
   fmt.Println("----------------------------------------------+-----------")
   for _, p := range providers {
      fmt.Printf("%-45s | %s\n", p.Slug, p.HasTitles)
   }
   fmt.Printf("\nTotal providers extracted: %d\n", len(providers))
}
