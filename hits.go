package main

import (
	"fmt"
	"sort"
)

type Hit struct {
	RegExId     int
	PageNum     int // PageIdx = PageNum - 1
	Pct         int
	Start, Stop int
	PageExtract string
}

type Hits map[int][]Hit

func (h *Hit) String() string {
	return fmt.Sprintf("%02v%%  %v\n", h.Pct, h.PageExtract)
}

// All hits for a particular regex - for the entire Pdf
func (h *Hits) RegExHits(RegExId int) Hits {
	ret := Hits{}
	hderef := *h
	for pageNum, pageHits := range hderef {
		for _, pageHit := range pageHits {
			if RegExId == pageHit.RegExId {
				ret[pageNum] = append(ret[pageNum], pageHit)
			}
		}
	}
	return ret
}

// Has hits for several denoted regexes - for specific page
func (h *Hits) HasRegExesHitsAtPage(PageNum int, RegExIds []int) bool {
	hderef := *h
	pageHits := hderef[PageNum]
	distinct := map[int]int{}
	for _, pageHit := range pageHits {
		for _, regExId := range RegExIds {
			if pageHit.RegExId == regExId {
				distinct[regExId]++
			}
		}
	}
	if len(distinct) == len(RegExIds) {
		return true
	} else {
		return false
	}
}

// Has hits for several denoted regexes - for the entire pdf - on the *same* page
func (h *Hits) HasRegExesHitsAtAnyOnePage(RegExIds []int) bool {
	hderef := *h
	for pageNum := range hderef {
		if h.HasRegExesHitsAtPage(pageNum, RegExIds) {
			return true
		}
	}
	return false
}

//
//

// All hits for a particular page - sorted by Pct
func (h *Hits) HitsPerPageSortedByPct(PageNum int) []Hit {

	hits := map[int]Hit{}
	keys := []int{}

	hderef := *h
	pageHits := hderef[PageNum]
	for _, pageHit := range pageHits {
		hits[pageHit.Pct] = pageHit
		keys = append(keys, pageHit.Pct)
	}

	sorted := []Hit{}
	sort.Ints(keys)
	for _, key := range keys {
		sorted = append(sorted, hits[key])
	}

	return sorted
}
