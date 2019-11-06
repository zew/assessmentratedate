package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	cus "google.golang.org/api/customsearch/v1"

	"github.com/kataras/iris/v12"

	"github.com/zew/decisiondates/config"
	"github.com/zew/decisiondates/gorpx"
	"github.com/zew/decisiondates/mdl"
	"github.com/zew/logx"
	"github.com/zew/util"
)

func customSearchServiceWrap(c iris.Context) *cus.Service {
	cseService, err := customSearchService()
	if err != nil {
		c.WriteString(err.Error())
		c.StopExecution()
	} else {
		logx.Printf("CSE client successfully retrieved")
	}
	return cseService
}

func customSearchService() (*cus.Service, error) {

	// Alternative way to get a client;
	// requires env GOOGLE_APPLICATION_CREDENTIALS=./app_service_account.json
	// Does *not* yield a custom search client.
	if false {
		client, err := google.DefaultClient(oauth2.NoContext)
		_, _ = client, err
	}

	//Get the config from the json key file with the correct scope
	// data, err := ioutil.ReadFile("app_service_account_lib_islands.json")
	data, err := ioutil.ReadFile(config.CredentialFileName(false))
	if err != nil {
		logx.Printf("#1\t%v", err)
		return nil, err
	}
	conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/cse")
	if err != nil {
		logx.Printf("#2\t%v", err)
		return nil, err
	}
	client := conf.Client(oauth2.NoContext)

	cses, err := cus.New(client)
	if err != nil {
		logx.Printf("#3\t%v", err)
		return nil, err
	}

	return cses, nil

}

func results(c iris.Context) {

	var err error
	display := ""
	respBytes := []byte{}
	strUrl := ""

	if EffectiveParam(c, "submit", "none") != "none" {

		start := EffectiveParamInt(c, "Start", 1)
		cnt := EffectiveParamInt(c, "Count", 5)
		end := start + cnt

		//
		//
		communities := []mdl.Community{}
		/*
			      community_id
			    , community_key
				, cleansed as community_name

		*/
		sql := `SELECT 
						*
			FROM 			` + gorpx.TableName(mdl.Community{}) + ` t1
			WHERE 			1=1
				AND		community_id >= :start_id
				AND		community_id <  :end_id
			`
		args := map[string]interface{}{
			"start_id": start,
			"end_id":   end,
		}
		_, err = gorpx.DBMap().Select(&communities, sql, args)
		util.CheckErr(err)
		for i := 0; i < len(communities); i++ {
			logx.Printf("%-4v  %-5v  %v\n", i, communities[i].Id, communities[i].Name)
		}

		cseService := customSearchServiceWrap(c)

	Label1:
		for i := 0; i < len(communities); i++ {

			display += fmt.Sprintf("============================\n")
			display += fmt.Sprintf("%v\n", communities[i].Name)

			// https://godoc.org/google.golang.org/api/customsearch/v1
			// CSE Limits you to 10 pages of results with max 10 results per page

			search := cseService.Cse.List(communities[i].Name)
			search.Cx("000184963688878042004:kcoarvtcg7q")
			// search.ExactTerms(communities[i].Key)

			// www.aeksh.de - aerztekammer
			search.ExcludeTerms("factfish www.aeksh.de")

			search.OrTerms("hebesätze hebesatz")
			search.FileType("pdf")
			search.Safe("off")
			start := int64(1)
			offset := int64(10)     // max allowed is 10
			maxResults := int64(30) // consuming up to three requests; sometimes only 4 results exist

			search.Start(start)
			search.Num(offset)

			for start < maxResults {
				search.Start(int64(start))
				call, err := search.Do()
				if err != nil {
					errStr := strings.ToLower(err.Error())
					if strings.Contains(errStr, "daily limit exceeded") {
						config.CredentialFileName(true)
						msg := fmt.Sprintf("CSE client needs re-initiation at community %v\n", i)
						logx.Printf(msg)
						display += msg
						// c.StopExecution()
						break Label1
					}
					c.Writef("search.Do: %s", err.Error())
					return
				}
				for index, r := range call.Items {
					display += fmt.Sprintf("%-4v %-22v %-32v  %v\n", start+int64(index), r.FileFormat, r.Link, r.DisplayLink)
					display += fmt.Sprintf("%v\n", r.Title)
					display += fmt.Sprintf("%v\n", r.Snippet)
					// display += fmt.Sprintf("%+v\n", r)
					display += fmt.Sprintf("\n")

					pdf := mdl.Pdf{}
					pdf.CommunityKey = communities[i].Key
					pdf.CommunityName = communities[i].Name
					pdf.Url = r.Link
					pdf.Title = r.Title
					pdf.ResultRank = int(start) + index
					pdf.SnippetGoogle = r.Snippet
					err = gorpx.DBMap().Insert(&pdf)
					if err != nil {
						c.WriteString(err.Error())
					}
				}
				start = start + offset
				// No more search results?
				if start > call.SearchInformation.TotalResults {
					break
				}
			}

		}
	}

	{
		sql := `
			/* update frequencies of pdf urls*/
			UPDATE
				pdf t1
				INNER JOIN (
					SELECT pdf_url, count(*) anz 
					FROM pdf t2
					GROUP BY pdf_url
			) t2 USING (pdf_url)
			SET t1.pdf_frequency = t2.anz
      `
		args := map[string]interface{}{}
		updateRes, err := gorpx.DBMap().Exec(sql, args)
		util.CheckErr(err)
		logx.Printf("updated frequencies: %+v\n", updateRes)

	}

	{
		sql := `
			/* remove pdf_text and snippets for noisy pdfs */
			UPDATE pdf
			SET pdf_snippet1= '', pdf_snippet2= '', pdf_snippet3= ''
			WHERE pdf_frequency > 2
      `
		args := map[string]interface{}{}
		updateRes, err := gorpx.DBMap().Exec(sql, args)
		util.CheckErr(err)
		logx.Printf("emptied : %+v\n", updateRes)

	}

	s := struct {
		HTMLTitle string
		Title     string
		Links     []struct{ Title, Url string }

		FormAction string
		Gemeinde   string
		Schluessel string
		ParamStart string
		ParamCount string

		Url    string
		UrlCmp string

		StructDump template.HTML
		RespBytes  template.HTML
	}{
		HTMLTitle: AppName() + " - Search for pdf docs on each community",
		Title:     AppName() + " - Search for pdf docs on each community",
		Links:     links,

		StructDump: template.HTML(display),
		RespBytes:  template.HTML(string(respBytes)),

		Url:        strUrl,
		UrlCmp:     "https://www.googleapis.com/customsearch/v1?q=Schwetzingen&key=AIzaSyDS56qRpWj3o_xfGqxwbP5oqW9qr72Poww&cx=000184963688878042004:kcoarvtcg7q",
		FormAction: PathCommunityResults,

		Gemeinde:   EffectiveParam(c, "Gemeinde", "Schwetzingen"),
		Schluessel: EffectiveParam(c, "Schluessel", "08 2 26 084"),
		ParamStart: EffectiveParam(c, "Start", "1"),
		ParamCount: EffectiveParam(c, "Count", "5"),
	}

	err = c.View("results.html", s)
	util.CheckErr(err)

}
