package main

import (
   "bytes"
   "encoding/json"
   "flag"
   "fmt"
   "io"
   "log"
   "net/http"
)

// ResponseData models the structure we need to extract totalCount
type ResponseData struct {
   Data struct {
      PopularTitles struct {
         TotalCount int `json:"totalCount"`
      } `json:"popularTitles"`
   } `json:"data"`
}

func main() {
   // 1. Define command line flags
   pkg := flag.String("package", "cpd", "JustWatch package code (e.g., cpd, nfx, hbm)")
   lang := flag.String("language", "cs", "Language code (e.g., cs, en)")
   country := flag.String("country", "CZ", "Country code (e.g., CZ, US)")
   flag.Parse()

   // 2. Construct the GraphQL Variables
   variables := map[string]interface{}{
      "first":               40,
      "popularTitlesSortBy": "POPULAR",
      "sortRandomSeed":      0,
      "offset":              0,
      "after":               "",
      "popularTitlesFilter": map[string]interface{}{
         "ageCertifications":          []string{},
         "excludeGenres":              []string{},
         "excludeProductionCountries": []string{},
         "objectTypes":                []string{},
         "productionCountries":        []string{},
         "subgenres":                  []string{},
         "genres":                     []string{},
         "packages":                   []string{*pkg},
         "excludeIrrelevantTitles":    false,
         "presentationTypes":          []string{},
         "monetizationTypes":          []string{},
         "searchQuery":                "",
      },
      "watchNowFilter": map[string]interface{}{
         "packages":          []string{*pkg},
         "monetizationTypes": []string{},
      },
      "language": *lang,
      "country":  *country,
   }

   // 3. Construct the full request payload
   payload := map[string]interface{}{
      "operationName": "GetPopularTitles",
      "variables":     variables,
      "query":         graphqlQuery,
   }

   jsonData, err := json.Marshal(payload)
   if err != nil {
      log.Fatalf("Error marshaling JSON payload: %v", err)
   }

   // 4. Create and configure the HTTP request
   req, err := http.NewRequest("POST", "https://apis.justwatch.com/graphql", bytes.NewBuffer(jsonData))
   if err != nil {
      log.Fatalf("Error creating request: %v", err)
   }

   // Adding headers mirroring the original request to avoid getting blocked
   req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0")
   req.Header.Set("Accept", "application/graphql-response+json,application/json;q=0.9")
   req.Header.Set("Accept-Language", "en-US,en;q=0.5")
   req.Header.Set("Content-Type", "application/json")
   req.Header.Set("Origin", "https://www.justwatch.com")
   req.Header.Set("Referer", "https://www.justwatch.com/")

   // 5. Execute the request
   client := &http.Client{}
   resp, err := client.Do(req)
   if err != nil {
      log.Fatalf("HTTP request failed: %v", err)
   }
   defer resp.Body.Close()

   bodyBytes, err := io.ReadAll(resp.Body)
   if err != nil {
      log.Fatalf("Failed to read response body: %v", err)
   }

   if resp.StatusCode != http.StatusOK {
      log.Fatalf("API returned non-200 status code: %d\nBody: %s", resp.StatusCode, string(bodyBytes))
   }

   // 6. Parse the response and print the totalCount
   var responseData ResponseData
   if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
      log.Fatalf("Error unmarshaling response JSON: %v", err)
   }

   fmt.Printf("Total Count: %d\n", responseData.Data.PopularTitles.TotalCount)
}

// Constant holding the large GraphQL query string
const graphqlQuery = `query GetPopularTitles($country: Country!, $first: Int! = 70, $format: ImageFormat, $language: Language!, $after: String, $popularTitlesFilter: TitleFilter, $popularTitlesSortBy: PopularTitlesSorting! = POPULAR, $profile: PosterProfile, $sortRandomSeed: Int! = 0, $watchNowFilter: WatchNowOfferFilter!, $offset: Int = 0) {
  popularTitles(
    country: $country
    filter: $popularTitlesFilter
    first: $first
    sortBy: $popularTitlesSortBy
    sortRandomSeed: $sortRandomSeed
    offset: $offset
    after: $after
  ) {
    __typename
    edges {
      cursor
      node {
        ...PopularTitleGraphql
        __typename
      }
      __typename
    }
    pageInfo {
      startCursor
      endCursor
      hasPreviousPage
      hasNextPage
      __typename
    }
    totalCount
  }
}


fragment PopularTitleGraphql on MovieOrShow {
  __typename
  id
  objectId
  objectType
  content(country: $country, language: $language) {
    title
    fullPath
    originalReleaseYear
    shortDescription
    interactions {
      likelistAdditions
      dislikelistAdditions
      __typename
    }
    scoring {
      imdbVotes
      imdbScore
      tmdbPopularity
      tmdbScore
      tomatoMeter
      certifiedFresh
      jwRating
      __typename
    }
    interactions {
      votesNumber
      __typename
    }
    posterUrl(profile: $profile, format: $format)
    isReleased
    runtime
    __typename
  }
  likelistEntry {
    createdAt
    __typename
  }
  dislikelistEntry {
    createdAt
    __typename
  }
  watchlistEntryV2 {
    createdAt
    __typename
  }
  customlistEntries {
    createdAt
    __typename
  }
  freeOffersCount: offerCount(
    country: $country
    platform: WEB
    filter: {monetizationTypes: [FREE, ADS]}
  )
  watchNowOffer(country: $country, platform: WEB, filter: $watchNowFilter) {
    ...WatchNowOffer
    __typename
  }
  ... on Movie {
    seenlistEntry {
      createdAt
      __typename
    }
    __typename
  }
  ... on Show {
    tvShowTrackingEntry {
      createdAt
      __typename
    }
    seenState(country: $country) {
      seenEpisodeCount
      progress
      __typename
    }
    __typename
  }
}


fragment WatchNowOffer on Offer {
  __typename
  id
  standardWebURL
  preAffiliatedStandardWebURL
  streamUrl
  streamUrlExternalPlayer
  package {
    id
    icon
    packageId
    clearName
    shortName
    technicalName
    iconWide(profile: S160)
    hasRectangularIcon(country: $country, platform: WEB)
    __typename
  }
  retailPrice(language: $language)
  retailPriceValue
  lastChangeRetailPriceValue
  currency
  presentationType
  monetizationType
  availableTo
  dateCreated
  newElementCount
  mediaDealId
}`
