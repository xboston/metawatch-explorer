package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
)

type Pagination struct {
	total       int64
	currentPage int64
	limit       int64
	urlPattern  string
	numLinks    int64
	numPages    int64
	start       int64
	end         int64
	StartLink   string  `json:"start_link"`
	EndLink     string  `json:"end_link"`
	Pages       []*Page `json:"pages"`
}

type Page struct {
	Active bool   `json:"active"`
	Number int64  `json:"number"`
	Link   string `json:"link"`
}

func NewPagination(total, currentPage, limit int64, urlPattern string) *Pagination {
	p := new(Pagination)
	p.total = total
	p.currentPage = currentPage
	p.limit = limit
	p.urlPattern = urlPattern + "%d"
	p.numLinks = 10
	return p
}

func (self *Pagination) SetNumLinks(numLinks int64) *Pagination {
	self.numLinks = numLinks
	return self
}

func (self *Pagination) Render() template.HTML {

	if self == nil {
		return template.HTML("")
	}

	// Create a new template and parse the letter into it.
	var out bytes.Buffer
	tPagination := template.Must(template.New("pagination").Parse(tmplPagination))
	tMap := map[string]interface{}{
		"links":       self.Pages,
		"StartLink":   self.StartLink,
		"EndLink":     self.EndLink,
		"numPages":    self.numPages,
		"currentPage": self.currentPage,
		"limit":       self.limit,
		"numLinks":    self.numLinks,
		"minShow":     (self.currentPage > (self.numLinks/2)+1),
		"maxShow":     (self.currentPage < ((self.numLinks) + self.numPages/2)) && self.numPages > self.numLinks,
	}

	_ = tPagination.Execute(&out, tMap)
	return template.HTML(out.String())
}

func (self *Pagination) Summary() template.HTML {
	// Create a new template and parse the letter into it.
	var out bytes.Buffer
	tSummary := template.Must(template.New("summary").Parse(tmplSummary))
	tMap := map[string]interface{}{
		"start": self.start,
		"end":   self.end,
		"total": self.total,
	}
	_ = tSummary.Execute(&out, tMap)
	return template.HTML(out.String())
}

func (self *Pagination) Init() {
	if self.currentPage < 1 {
		self.currentPage = 1
	}

	if self.limit == 0 {
		self.limit = 10
	}

	numPages := int64(math.Ceil(float64(self.total) / float64(self.limit)))
	self.numPages = numPages
	if numPages > 1 {
		self.StartLink = fmt.Sprintf(self.urlPattern, 1)
		self.EndLink = fmt.Sprintf(self.urlPattern, numPages)

		if numPages < self.numLinks {
			self.start = 1
			self.end = numPages
		} else {
			self.start = self.currentPage - int64(math.Floor(float64(self.numLinks)/float64(2)))
			self.end = self.currentPage + int64(math.Floor(float64(self.numLinks)/float64(2)))

			if self.start < 1 {
				self.end += int64(math.Abs(float64(self.start))) + 1
				self.start = 1
			}

			if self.end > numPages {
				self.start -= (self.end - numPages) - 1
				self.end = numPages
			}
		}

		for i := self.start; i <= self.end; i++ {
			page := new(Page)
			page.Number = i
			page.Link = fmt.Sprintf(self.urlPattern, page.Number)
			if page.Number == self.currentPage {
				page.Active = true
			} else {
				page.Active = false
			}
			self.Pages = append(self.Pages, page)
		}
	}

	if self.total > 0 {
		self.start = ((self.currentPage - 1) * self.limit) + 1
	}

	if ((self.currentPage - 1) * self.limit) > (self.total - self.limit) {
		self.end = self.total
	} else {
		self.end = ((self.currentPage - 1) * self.limit) + self.limit
	}
}

const (
	tmplPagination string = `
{{if .links}}
<nav class="pagination is-right is-small">
<!--
<a class="pagination-previous">Previous</a>
<a class="pagination-next">Next page</a>
-->
<ul class="pagination-list">
	{{ if .minShow}}
		<li><a class="pagination-link" href="{{.StartLink}}">1</a></li>
		<li><span class="pagination-ellipsis">&hellip;</span></li>
	{{end}}

	{{range .links}}
	{{ if .Active }}
		<li><span class="pagination-link is-current">{{.Number}}</span></li>
	{{ else }}
		<li><a class="pagination-link" href="{{.Link}}">{{.Number}}</a></li>
	{{ end }}
	{{end}}

	{{ if .maxShow}}
		<li><span class="pagination-ellipsis">&hellip;</span></li>
		<li><a class="pagination-link" href="{{.EndLink}}">{{.numPages}}</a></li>
	{{ end }}
</li>
</ul>
</nav>
{{end}}
`
	tmplSummary string = `{{if .total}}<div class="summary text-right">Displaying {{.start}}-{{.end}} of {{.total}} results.</div>{{end}}`
)
