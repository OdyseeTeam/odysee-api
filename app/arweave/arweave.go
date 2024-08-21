package arweave

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func ReplaceAssetUrls(baseUrl string, structure any, collPath, itemPath string) (any, error) {
	var origUrls []string
	urlPaths := map[string][]string{}

	jsonData, err := json.Marshal(structure)
	if err != nil {
		return nil, err
	}

	items := gjson.GetBytes(jsonData, collPath)
	items.ForEach(func(key, value gjson.Result) bool {
		urlPath := fmt.Sprintf("%s.%s.%s", collPath, key.String(), itemPath)
		url := gjson.GetBytes(jsonData, urlPath).String()
		origUrls = append(origUrls, url)
		if slice, exists := urlPaths[url]; exists {
			urlPaths[url] = append(slice, urlPath)
		} else {
			urlPaths[url] = []string{urlPath}
		}
		return true
	})

	resolver := NewAssetResolver(baseUrl)
	subsUrls, err := resolver.ResolveUrls(origUrls)
	if err != nil {
		return nil, err
	}

	for oldURL, newURL := range subsUrls {
		for _, path := range urlPaths[oldURL] {
			jsonData, _ = sjson.SetBytes(jsonData, path, newURL)
		}
	}

	var d any
	return d, json.Unmarshal(jsonData, &d)
}

func ReplaceAssetUrl(baseUrl string, structure any, path string) (any, error) {
	jsonData, err := json.Marshal(structure)
	if err != nil {
		return nil, err
	}

	origUrl := gjson.GetBytes(jsonData, path).String()

	resolver := NewAssetResolver(baseUrl)
	subsUrls, err := resolver.ResolveUrls([]string{origUrl})

	if err != nil {
		return nil, err
	}
	if newUrl, ok := subsUrls[origUrl]; ok {
		jsonData, err = sjson.SetBytes(jsonData, path, newUrl)
		if err != nil {
			return nil, err
		}
	}

	var d any
	return d, json.Unmarshal(jsonData, &d)
}

func GetClaimUrl(baseUrl, claim_id string) (string, error) {
	resolver := NewAssetResolver(baseUrl)
	r, err := resolver.ResolveClaims([]string{claim_id})
	if err != nil {
		return "", err
	}
	return r[claim_id], nil
}
