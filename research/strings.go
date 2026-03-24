package main

import (
   "fmt"
   "io"
   "log"
   "os"
   "sort"
   "strings"
)

type Provider struct {
   Slug      string
   HasTitles string
}

type tempProvider struct {
   Slug      string
   HasTitles string
}

// extractVarName walks backwards from the end of the `before` string
// to isolate the variable name (e.g., in `...{some code};a`, it extracts "a")
func extractVarName(before string) string {
   for i := len(before) - 1; i >= 0; i-- {
      c := before[i]
      isAlphanum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
      if !isAlphanum {
         return before[i+1:]
      }
   }
   return before
}

func main() {
   file, err := os.Open("input.txt")
   if err != nil {
      log.Fatalf("Failed to open file: %v", err)
   }
   defer file.Close()

   content, err := io.ReadAll(file)
   if err != nil {
      log.Fatalf("Failed to read file: %v", err)
   }

   text := string(content)
   providerMap := make(map[string]*tempProvider)

   // --- 1. Find all `.hasTitles=` assignments ---
   remaining := text
   for {
      before, after, found := strings.Cut(remaining, ".hasTitles=")
      if !found {
         break
      }

      varName := extractVarName(before)
      if varName != "" {
         if providerMap[varName] == nil {
            providerMap[varName] = &tempProvider{}
         }
         // The value is immediately at the start of `after`
         if strings.HasPrefix(after, "true") {
            providerMap[varName].HasTitles = "true"
         } else if strings.HasPrefix(after, "false") {
            providerMap[varName].HasTitles = "false"
         }
      }
      remaining = after // continue searching in the remaining text
   }

   // --- 2. Find all `.slug="` assignments ---
   remaining = text
   for {
      before, after, found := strings.Cut(remaining, ".slug=\"")
      if !found {
         break
      }

      varName := extractVarName(before)
      if varName != "" {
         // Extract the slug value by cutting at the closing quote
         slugVal, _, foundQuote := strings.Cut(after, "\"")
         if foundQuote {
            if providerMap[varName] == nil {
               providerMap[varName] = &tempProvider{}
            }
            providerMap[varName].Slug = slugVal
         }
      }
      remaining = after // continue searching in the remaining text
   }

   // --- 3. Filter valid pairs and sort ---
   var providers[]Provider
   for _, data := range providerMap {
      if data.Slug != "" && data.HasTitles != "" {
         providers = append(providers, Provider{
            Slug:      data.Slug,
            HasTitles: data.HasTitles,
         })
      }
   }

   sort.Slice(providers, func(i, j int) bool {
      return providers[i].Slug < providers[j].Slug
   })

   // --- 4. Print Results ---
   fmt.Println("Extracted Providers:")
   fmt.Printf("%-45s | %s\n", "Slug", "HasTitles")
   fmt.Println("----------------------------------------------+-----------")
   for _, p := range providers {
      fmt.Printf("%-45s | %s\n", p.Slug, p.HasTitles)
   }
   fmt.Printf("\nTotal providers extracted: %d\n", len(providers))
}
