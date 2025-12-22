package output

import (
	"fmt"
	"os"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	urlutil "github.com/law-makers/crawl/internal/utils/url"
	"github.com/law-makers/crawl/pkg/models"
)

// SaveMarkdown converts HTML to Markdown and writes it to filepath
func SaveMarkdown(data *models.PageData, filepath string) error {
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())

	// Add rule to resolve relative links
	converter.AddRules(md.Rule{
		Filter: []string{"a"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			href, exists := selec.Attr("href")
			if !exists {
				return nil
			}

			resolved := urlutil.ResolveURL(data.URL, href)
			title, hasTitle := selec.Attr("title")
			var titlePart string
			if hasTitle {
				titlePart = fmt.Sprintf(" %q", title)
			}
			str := fmt.Sprintf("[%s](%s)%s", selec.Text(), resolved, titlePart)
			return &str
		},
	})

	cleaned, err := CleanHTML(data.HTML)
	if err != nil {
		return err
	}

	mdStr, err := converter.ConvertString(cleaned)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, []byte(mdStr), 0644)
}
