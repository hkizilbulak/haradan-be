package tjk

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

func parseBulkSummary(body []byte) ([]Horse, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, permanentErr("parse TJK HTML failed", 0)
	}
	if horses := parseBulkLinkSequence(doc); len(horses) > 0 {
		return horses, nil
	}
	if horses := parseBulkTableRows(doc); len(horses) > 0 {
		return horses, nil
	}
	// Empty page is a valid end-of-listing signal for the worker.
	return []Horse{}, nil
}

// parseBulkLinkSequence mirrors legacy HorseService.parseHorseSummary: walk
// nodes for QueryParameter_AtId / BabaAdi / AnneAdi anchors. Malformed items
// are skipped without aborting the batch.
func parseBulkLinkSequence(doc *html.Node) []Horse {
	var out []Horse
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			switch {
			case containsFold(href, "QueryParameter_AtId"):
				id := extractAtID(href)
				name := normalizedText(n)
				if id != "" && name != "" {
					h := Horse{Number: id, Name: name, Race: raceAfterAnchor(n)}
					out = append(out, h)
				}
			case containsFold(href, "QueryParameter_BabaAdi"):
				if len(out) > 0 {
					out[len(out)-1].Sire = normalizedText(n)
				}
			case containsFold(href, "QueryParameter_AnneAdi"):
				if len(out) > 0 {
					out[len(out)-1].Dam = normalizedText(n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func raceAfterAnchor(n *html.Node) string {
	// Prefer the first non-empty text sibling before the next query link.
	for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode && sib.Data == "a" {
			href := attr(sib, "href")
			if containsFold(href, "QueryParameter_") || queryHasAtID(href) {
				break
			}
		}
		if sib.Type == html.TextNode {
			if text := strings.TrimSpace(sib.Data); text != "" {
				return text
			}
		}
	}
	// Legacy fallback: third nextSibling string.
	sib := n.NextSibling
	for i := 0; i < 3 && sib != nil; i++ {
		if i == 2 {
			return strings.TrimSpace(flatNodeText(sib))
		}
		sib = sib.NextSibling
	}
	return ""
}

func flatNodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	return normalizedText(n)
}

func parseBulkTableRows(doc *html.Node) []Horse {
	var out []Horse
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if h, ok := parseTableRow(n); ok {
				out = append(out, h)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func parseTableRow(row *html.Node) (Horse, bool) {
	var cells []string
	var number string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "td" {
			cells = append(cells, normalizedText(n))
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			if id := extractAtID(attr(n, "href")); id != "" {
				number = id
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(row)
	if number == "" || len(cells) < 2 {
		return Horse{}, false
	}
	h := Horse{Number: number, Name: cells[1]}
	if len(cells) > 2 {
		h.Race = cells[2]
	}
	if len(cells) > 3 {
		h.Sire = cells[3]
	}
	if len(cells) > 4 {
		h.Dam = cells[4]
	}
	return h, h.Name != ""
}

func parseDetail(body []byte) (Detail, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return Detail{}, permanentErr("parse TJK detail HTML failed", 0)
	}
	var d Detail
	for _, spanTexts := range collectGridSpanPairs(doc, "grid_8") {
		applyDetailPair(&d, spanTexts[0], spanTexts[1])
	}
	d.Statistics = parseStatisticRows(doc)
	return d, nil
}

func collectGridSpanPairs(doc *html.Node, className string) [][2]string {
	var pairs [][2]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, className) {
			var spans []string
			var collect func(*html.Node)
			collect = func(x *html.Node) {
				if x.Type == html.ElementNode && x.Data == "span" {
					spans = append(spans, normalizedText(x))
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					collect(c)
				}
			}
			collect(n)
			for i := 0; i+1 < len(spans); i += 2 {
				pairs = append(pairs, [2]string{spans[i], spans[i+1]})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return pairs
}

func applyDetailPair(d *Detail, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	switch key {
	case "İsim":
		d.Name = strings.TrimSpace(strings.ReplaceAll(value, "(Öldü)", ""))
	case "Yaş":
		d.AgeText = value
	case "Doğ. Trh":
		d.BirthDate = value
	case "Handikap P.":
		d.HandicapPoint = value
	case "Baba":
		d.Sire = value
	case "Anne":
		// Legacy: "Anne / Maidensire" — keep dam name only in typed Dam.
		if parts := strings.SplitN(value, "/", 2); len(parts) > 0 {
			d.Dam = strings.TrimSpace(parts[0])
		} else {
			d.Dam = value
		}
	case "Gerçek Sahip":
		d.Owner = value
	case "Yetiştirici":
		d.Grower = value
	}
}

func parseStatisticRows(doc *html.Node) []RaceStatistic {
	var out []RaceStatistic
	for _, table := range findTablesUnderClass(doc, "grid_10") {
		for _, row := range tableBodyRows(table) {
			cells := rowCellTexts(row)
			if len(cells) == 0 {
				continue
			}
			s := RaceStatistic{}
			if len(cells) > 0 {
				s.YearLabel = cells[0]
			}
			if len(cells) > 1 {
				s.RaceCount = cells[1]
			}
			if len(cells) > 2 {
				s.First = cells[2]
			}
			if len(cells) > 3 {
				s.Second = cells[3]
			}
			if len(cells) > 4 {
				s.Third = cells[4]
			}
			if len(cells) > 5 {
				s.Fourth = cells[5]
			}
			if len(cells) > 6 {
				s.Fifth = cells[6]
			}
			if len(cells) > 7 {
				s.Earning = cells[7]
			}
			out = append(out, s)
		}
	}
	return out
}

func parsePedigree(body []byte) ([]PedigreeEntry, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, permanentErr("parse TJK pedigree HTML failed", 0)
	}
	// Legacy allocates 7 Pedigri slots and fills by rowspan.
	entries := make([]PedigreeEntry, 7)
	i, j := 1, 3
	for _, table := range findTablesUnderClass(doc, "grid_24") {
		for _, row := range tableBodyRows(table) {
			for td := firstChildElement(row, "td"); td != nil; td = nextSiblingElement(td, "td") {
				raw := nodeHTMLHint(td)
				text := normalizedText(td)
				switch {
				case strings.Contains(raw, `rowspan="4"`):
					setPedigreeParent(&entries[0], td, text)
				case strings.Contains(raw, `rowspan="2"`):
					if setPedigreeParent(&entries[i], td, text) {
						i++
						if i >= len(entries) {
							i = len(entries) - 1
						}
					}
				default:
					if setPedigreeParent(&entries[j], td, text) {
						j++
						if j >= len(entries) {
							j = len(entries) - 1
						}
					}
				}
			}
		}
	}
	// Drop trailing completely empty slots.
	for len(entries) > 0 && entries[len(entries)-1].Father == "" && entries[len(entries)-1].Mother == "" {
		entries = entries[:len(entries)-1]
	}
	return entries, nil
}

func setPedigreeParent(e *PedigreeEntry, td *html.Node, text string) bool {
	// Legacy: style containing #dbdbdb marks father; otherwise mother completes the pair.
	if strings.Contains(nodeHTMLHint(td), "#dbdbdb") {
		e.Father = text
		return false
	}
	e.Mother = text
	return true
}

func parseSiblings(body []byte) ([]Sibling, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, permanentErr("parse TJK sibling HTML failed", 0)
	}
	var out []Sibling
	for _, table := range findTablesUnderClass(doc, "grid_24") {
		for _, row := range tableBodyRows(table) {
			cells := rowCellTexts(row)
			if len(cells) == 0 {
				continue
			}
			s := Sibling{}
			if len(cells) > 0 {
				s.Name = cells[0]
			}
			if len(cells) > 1 {
				s.FatherName = cells[1]
			}
			if len(cells) > 2 {
				s.RaceCount = cells[2]
			}
			if len(cells) > 3 {
				s.First = cells[3]
			}
			if len(cells) > 4 {
				s.Second = cells[4]
			}
			if len(cells) > 5 {
				s.Third = cells[5]
			}
			if len(cells) > 6 {
				s.Fourth = cells[6]
			}
			if len(cells) > 7 {
				s.Earning = cells[7]
			}
			if s.Name == "" {
				continue
			}
			out = append(out, s)
		}
	}
	return out, nil
}

func findTablesUnderClass(doc *html.Node, className string) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, className) {
			var find func(*html.Node)
			find = func(x *html.Node) {
				if x.Type == html.ElementNode && x.Data == "table" {
					tables = append(tables, x)
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					find(c)
				}
			}
			find(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return tables
}

func tableBodyRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" && !hasTheadAncestor(n) {
			rows = append(rows, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(table)
	return rows
}

func hasTheadAncestor(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "thead" {
			return true
		}
	}
	return false
}

func rowCellTexts(row *html.Node) []string {
	var cells []string
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			cells = append(cells, normalizedText(c))
		}
	}
	return cells
}

func firstChildElement(n *html.Node, name string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			return c
		}
	}
	return nil
}

func nextSiblingElement(n *html.Node, name string) *html.Node {
	for c := n.NextSibling; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			return c
		}
	}
	return nil
}

func nodeHTMLHint(n *html.Node) string {
	// Lightweight attribute serialization for rowspan/style checks (no full render).
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(n.Data)
	for _, a := range n.Attr {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteString(`="`)
		b.WriteString(a.Val)
		b.WriteByte('"')
	}
	b.WriteByte('>')
	return b.String()
}

func extractAtID(href string) string {
	u, err := url.Parse(href)
	if err == nil {
		for key, values := range u.Query() {
			if len(values) == 0 {
				continue
			}
			if strings.EqualFold(key, "AtId") || strings.EqualFold(key, "QueryParameter_AtId") {
				if v := strings.TrimSpace(values[0]); v != "" {
					return v
				}
			}
		}
	}
	// Legacy split fallback: "...AtId=<id>"
	lower := strings.ToLower(href)
	if idx := strings.Index(lower, "atid="); idx >= 0 {
		rest := href[idx+len("atid="):]
		if end := strings.IndexAny(rest, "&#\"'"); end >= 0 {
			rest = rest[:end]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

func queryHasAtID(href string) bool {
	return extractAtID(href) != ""
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, className string) bool {
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == className {
			return true
		}
	}
	return false
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func normalizedText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
