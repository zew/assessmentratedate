package main

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"

	"github.com/kataras/iris/v12"

	"github.com/zew/decisiondates/gorpx"
	"github.com/zew/decisiondates/mdl"
	"github.com/zew/logx"
	"github.com/zew/util"
)

func refineTextMultiPass(c iris.Context) {

	var err error
	display := ""
	strUrl := ""

	rs := []*regexp.Regexp{}

	r0, err := regexp.Compile("Hebes[aä]{1}tz[e]{0,1}")
	util.CheckErr(err)
	rs = append(rs, r0)

	// [\n\s]+  would be new line or space
	r1, err := regexp.Compile(`[^0-9]{1}[1-5]{1}[0-9]{1}[05]{1}[^0-9]{1}`)
	util.CheckErr(err)
	rs = append(rs, r1)

	str1 := "amtliche Bekanntmachung|Amtsblatt|Anzeiger|Bürgerhaushalt"
	str2 := "Gewerbesteuer|Gemeindeanzeiger|Gemeindeblatt|Gemeinderatsbeschluß|Grundsteuer"
	str3 := "Haushaltrede|Haushaltsdokument|Haushaltsplan|Haushaltssanierungsplan|Haushaltssatzung|Hebesatzsatzung"
	str4 := "Jahresabschluss|Jahresabschluß|Mitteilungsblatt|Nachhaltigkeitssatzung"
	str5 := "Protokoll|Sitzung|Stadtanzeiger"
	all2 := fmt.Sprintf("(?i)%v|%v|%v|%v|%v", str1, str2, str3, str4, str5)
	r3, err := regexp.Compile(all2)
	util.CheckErr(err)
	rs = append(rs, r3)

	//
	// original regex: ("[0-9]{2}[./ ]+[0-9]{2}[./ ]+[0-9]{4}")
	monthDays := "01|02|03|04|05|06|07|08|09|1|2|3|4|5|6|7|8|9|10|11|12|13|14|15|16|17|18|19|20|21|22|23|24|25|26|27|28|29|30|31"
	monthsShort := "Jan|Feb|Mrz|Apr|Mai|Jun|Jul|Aug|Sept|Sep|Okt|Nov|Dez"
	monthsLong := "Januar|Februar|März|April|Mai|Juni|Juli|August|September|Oktober|November|Dezember"
	monthsNumbered := "01|02|03|04|05|06|07|08|09|1|2|3|4|5|6|7|8|9|10|11|12"
	yearsLong := "2010|2011|2012|2013|2014|2015|2016"
	all3 := fmt.Sprintf("((%v)[./\\s]+(%v|%v|%v)[./\\s]+(%v))[^0-9]+", monthDays, monthsShort, monthsLong, monthsNumbered, yearsLong)
	r4, err := regexp.Compile(all3)
	util.CheckErr(err)
	rs = append(rs, r4)

	//
	if EffectiveParam(c, "submit", "none") != "none" {

		start := EffectiveParamInt(c, "Start", 1)
		cnt := EffectiveParamInt(c, "Count", 5)
		end := start + cnt

		//
		//
		pdfs := []mdl.Pdf{}
		sql := `SELECT 
					*
			FROM 			` + gorpx.TableName(mdl.Pdf{}) + ` t1
			WHERE 			1=1
				AND		pdf_id >= :start_id
				AND		pdf_id <  :end_id
				AND		pdf_frequency <= :frequency

			`
		args := map[string]interface{}{
			"start_id":  start,
			"end_id":    end,
			"frequency": maxFrequency,
		}
		_, err = gorpx.DBMap().Select(&pdfs, sql, args)
		util.CheckErr(err)

		for i := 0; i < len(pdfs); i++ {

			pdf := pdfs[i]
			hits := Hits{}

			pages := []mdl.Page{}
			sql := `SELECT 	*
			FROM 			` + gorpx.TableName(mdl.Page{}) + ` t1
			WHERE 			1=1
				AND		pdf_url = :pdf_url   `
			args := map[string]interface{}{
				"pdf_url": pdf.Url,
			}
			_, err = gorpx.DBMap().Select(&pages, sql, args)
			util.CheckErr(err)

			pdf.Snippet1 = ""
			pdf.Snippet2 = ""
			pdf.Snippet3 = ""

			for j := 0; j < len(pages); j++ {

				p := pages[j]

				{
					matchPos := rs[0].FindAllStringIndex(p.Content, -1)
					// pdf.Snippet1 = fmt.Sprintf("%v", matchPos)
					for _, occurrence := range matchPos {
						h := Hit{}
						h.PageNum = p.Number
						h.RegExId = 0
						h.Start = occurrence[0]
						h.Stop = occurrence[1]
						h.Pct = 100 * occurrence[0] / len(p.Content)
						h.PageExtract = snippetIt(occurrence, p.Content, 20, 110)
						hits[p.Number] = append(hits[p.Number], h)
					}
				}

				if hits.HasRegExesHitsAtPage(p.Number, []int{0}) {
					matchPos := rs[1].FindAllStringIndex(p.Content, -1)
					for _, occurrence := range matchPos {
						pos := occurrence[0]
						all0Hits := hits.RegExHits(0)
						for _, curPage0Hit := range all0Hits[p.Number] {
							distance := pos - curPage0Hit.Start
							if distance < 200 && distance > -20 {
								h := Hit{}
								h.PageNum = p.Number
								h.RegExId = 1
								h.Start = occurrence[0]
								h.Stop = occurrence[1]
								h.Pct = 100 * occurrence[0] / len(p.Content)
								h.PageExtract = snippetIt(occurrence, p.Content, 20, 110)
								hits[p.Number] = append(hits[p.Number], h)
							}
						}
					}
				}

			}

			// And again for nearby dates
			for j := 0; j < len(pages); j++ {

				p := pages[j]

				if hits.HasRegExesHitsAtPage(p.Number, []int{0, 1}) ||
					hits.HasRegExesHitsAtPage(p.Number-1, []int{0, 1}) ||
					hits.HasRegExesHitsAtPage(p.Number+1, []int{0, 1}) ||
					false {

					matchPosAll := rs[3].FindAllStringSubmatchIndex(p.Content, -1)
					for _, occurrence := range matchPosAll {
						h := Hit{}
						h.PageNum = p.Number
						h.RegExId = 3
						h.Start = occurrence[2] // the second sub-match; occurrence[2:4]
						h.Stop = occurrence[3]
						h.Pct = 100 * occurrence[2] / len(p.Content)
						h.PageExtract = snippetIt(occurrence[2:4], p.Content, 20, 110)
						hits[p.Number] = append(hits[p.Number], h)
					}
				}

			}

			// Now assemble display output
			if hits.HasRegExesHitsAtAnyOnePage([]int{0, 1}) {
				display += fmt.Sprintf("<a href='%v' target='pdf'>%v: %v</a>\n",
					pdf.Url, pdf.CommunityName, pdf.Title)

				for j := 0; j < len(pages); j++ {
					p := pages[j]
					if hits.HasRegExesHitsAtPage(p.Number, []int{0, 1}) {

						display += fmt.Sprintf("<a href='%v#page=%v' target='pdf'>Seite %02v</a>\n", pdf.Url, p.Number, p.Number)
						display += fmt.Sprintf(`<a onclick="openDecision(%v,%v);" href="javascript:void(0);"  >Decide</a>`,
							p.Id, pdf.Id)
						display += "\n"
						hitsByPct := hits.HitsPerPageSortedByPct(p.Number)
						lastTypes := []int{}
						for _, hitByPct := range hitsByPct {
							if !Repetitive(lastTypes, hitByPct.RegExId) {
								display += util.Ellipsoider(hitByPct.String(), 1800)
							} else {
								// display += "... \n"
							}
							lastTypes = append(lastTypes, hitByPct.RegExId)
							pdf.Snippet2 += util.Ellipsoider(hitByPct.String(), 1800)
						}
						display += "\n"
					}
				}
				display += "<hr/>\n\n"
			}

			//
			//
			numRows, err := gorpx.DBMap().Update(&pdf)
			if err != nil {
				display += fmt.Sprintf("Error during update: %v \n%v", err, &pdf.Snippet2)
				continue
			}
			if numRows > 0 {
				logx.Printf("%v rows updated; pdf_id %-5v", numRows, pdf.Id)
			}

		}
		logx.Printf("---------text refinement finished for--%v-%v-----", start, end)

	}

	s := struct {
		HTMLTitle string
		Title     string
		Links     []struct{ Title, Url string }

		FormAction string
		ParamStart string
		ParamCount string

		Url    string
		UrlCmp string

		StructDump template.HTML
		RespBytes  template.HTML
	}{
		HTMLTitle: AppName() + " - Refine possible matches",
		Title:     AppName() + " - Refine possible matches",
		Links:     links,

		StructDump: template.HTML(display),
		// RespBytes:  template.HTML(string(respBytes)),

		Url:        strUrl,
		FormAction: RefineTextMultiPass,

		ParamStart: EffectiveParam(c, "Start", "0"),
		ParamCount: EffectiveParam(c, "Count", "3"),
	}

	err = c.View("results.html", s)
	util.CheckErr(err)

}

func snippetIt(occurrence []int, haystack string, before int, after int) string {

	l := len(haystack)
	_ = l
	start := occurrence[0]
	stop := occurrence[1]

	start -= before
	if start < 0 {
		start = 0
	}

	stop += after
	if stop > l {
		stop = l
	}

	ret := bytes.Buffer{}
	// looping over possibly invalid utf-8 sequences
	// "... If the iteration encounters an invalid UTF-8 sequence, the second value will be 0xFFFD, ...""
	cnt := 0
	max := before + occurrence[1] - occurrence[0] + after
	for idx, codepoint := range haystack[start:stop] {

		if idx == before {
			// ret.WriteRune(rune(32)) // enclose into extra spaces
			ret.WriteString("<b>")
		}
		if idx == before+occurrence[1]-occurrence[0] {
			ret.WriteString("</b> ")
		}

		ret.WriteRune(codepoint)

		cnt++
		if cnt > max {
			break
		}
	}
	ret.WriteString("</b> ")

	return ret.String()

}

func Repetitive(RegExIds []int, RegExId int) bool {
	cnt := 0
	for i := len(RegExIds) - 1; i > -1; i-- {
		if RegExIds[i] == RegExId {
			cnt++
			if cnt > 2 {
				return true
			}
		} else {
			return false
		}
	}
	return false
}
