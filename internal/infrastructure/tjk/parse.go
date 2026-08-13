package tjk

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var totalCountPattern = regexp.MustCompile(`^Toplam\s+([0-9]+)(?:\s|$)`)

func parseBulkSummary(body []byte) (BulkPage, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return BulkPage{}, transientErr("parse TJK HTML failed", 0)
	}
	total := parseBulkTotal(doc)
	if horses, candidates := parseBulkLinkSequence(doc); candidates > 0 {
		return classifiedBulkPage(body, horses, candidates-len(horses), total)
	}
	if horses, candidates := parseBulkTableRows(doc); candidates > 0 {
		return classifiedBulkPage(body, horses, candidates-len(horses), total)
	}
	if total != nil && *total == 0 {
		return BulkPage{EndOfSource: true, SourceTotal: total}, nil
	}
	return BulkPage{}, transientErr("unrecognized TJK horse page", 0)
}

func classifiedBulkPage(body []byte, horses []Horse, skipped int, total *int) (BulkPage, error) {
	if total != nil && *total == 0 {
		return BulkPage{}, transientErr("inconsistent TJK horse page", 0)
	}
	h := sha256.New()
	if len(horses) == 0 {
		_, _ = h.Write(body)
	} else {
		for _, horse := range horses {
			_, _ = h.Write([]byte(horse.Number))
			_, _ = h.Write([]byte{0})
		}
	}
	return BulkPage{
		Horses: horses, Fingerprint: hex.EncodeToString(h.Sum(nil)),
		SourceTotal: total, SkippedCount: skipped,
	}, nil
}

func parseBulkTotal(doc *html.Node) *int {
	var total *int
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if total != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" {
			if match := totalCountPattern.FindStringSubmatch(normalizedText(n)); len(match) == 2 {
				if value, err := strconv.Atoi(match[1]); err == nil && value >= 0 {
					total = &value
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return total
}

// parseBulkLinkSequence mirrors legacy HorseService.parseHorseSummary: walk
// nodes for QueryParameter_AtId / BabaAdi / AnneAdi anchors. Malformed items
// are skipped without aborting the batch.
func parseBulkLinkSequence(doc *html.Node) ([]Horse, int) {
	var out []Horse
	candidates := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			switch {
			case containsFold(href, "QueryParameter_AtId"):
				candidates++
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
	return out, candidates
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

func parseBulkTableRows(doc *html.Node) ([]Horse, int) {
	var out []Horse
	candidates := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if h, ok, candidate := parseTableRow(n); candidate {
				candidates++
				if ok {
					out = append(out, h)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out, candidates
}

func parseTableRow(row *html.Node) (Horse, bool, bool) {
	var cells []string
	var number string
	var horseLink bool
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "td" {
			cells = append(cells, normalizedText(n))
		}
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			// Generic provider/error pages may also contain ordinary two-column
			// tables. Treat a row as a horse candidate only when it carries the
			// provider's horse-id link contract.
			if containsFold(href, "AtId=") {
				horseLink = true
			}
			if id := extractAtID(href); id != "" {
				number = id
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(row)
	if len(cells) < 2 || !horseLink {
		return Horse{}, false, false
	}
	if number == "" {
		return Horse{}, false, true
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
	return h, h.Name != "", true
}

func parseDetail(body []byte) (Detail, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return Detail{}, permanentErr("parse TJK detail HTML failed", 0)
	}
	var d Detail
	pairs := collectGridSpanPairs(doc, "grid_8")
	statTables := findTablesUnderClass(doc, "grid_10")
	if len(pairs) == 0 && len(statTables) == 0 {
		return Detail{}, transientErr("unrecognized TJK detail page", 0)
	}
	for _, spanTexts := range pairs {
		applyDetailPair(&d, spanTexts[0], spanTexts[1])
	}
	d.Statistics = parseStatisticRows(statTables)
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
		if parts := strings.SplitN(value, "/", 2); len(parts) > 0 {
			d.Dam = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				d.MaidenSire = strings.TrimSpace(parts[1])
			}
		}
	case "Gerçek Sahip":
		d.Owner = value
	case "Yetiştirici":
		d.Grower = value
	case "Kazanç":
		d.Earning = value
	}
}

func parseStatisticRows(tables []*html.Node) []RaceStatistic {
	var out []RaceStatistic
	for _, table := range tables {
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
	// Keep the legacy rowspan-to-pair ordering, but grow dynamically. The old
	// fixed seven slots silently overwrote the final entry when the provider
	// returned a deeper structure.
	tables := findTablesUnderClass(doc, "grid_24")
	if len(tables) == 0 {
		return nil, transientErr("unrecognized TJK pedigree page", 0)
	}
	entries := make([]PedigreeEntry, 0, 7)
	i, j := 1, 3
	for _, table := range tables {
		for _, row := range tableBodyRows(table) {
			for td := firstChildElement(row, "td"); td != nil; td = nextSiblingElement(td, "td") {
				raw := nodeHTMLHint(td)
				text := normalizedText(td)
				switch {
				case strings.Contains(raw, `rowspan="4"`):
					ensurePedigreeEntry(&entries, 0)
					setPedigreeParent(&entries[0], td, text)
				case strings.Contains(raw, `rowspan="2"`):
					ensurePedigreeEntry(&entries, i)
					if setPedigreeParent(&entries[i], td, text) {
						i++
					}
				default:
					ensurePedigreeEntry(&entries, j)
					if setPedigreeParent(&entries[j], td, text) {
						j++
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

func ensurePedigreeEntry(entries *[]PedigreeEntry, index int) {
	for len(*entries) <= index {
		*entries = append(*entries, PedigreeEntry{})
	}
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
	tables := findTablesUnderClass(doc, "grid_24")
	if len(tables) == 0 {
		// The live provider uses this exact, provider-owned response when the
		// horse authoritatively has no same-dam siblings. Unlike an arbitrary
		// empty 200, it is safe to replace stale sibling data with an empty list.
		if hasExactElementText(doc, "span", "Bu atın aynı anneden kardeşi yoktur.") {
			return []Sibling{}, nil
		}
		return nil, transientErr("unrecognized TJK sibling page", 0)
	}
	var out []Sibling
	for _, table := range tables {
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

func parseMating(body []byte) ([]MatingEntry, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, permanentErr("parse TJK mating HTML failed", 0)
	}
	tables := findTablesUnderClass(doc, "grid_24")
	if len(tables) == 0 {
		return []MatingEntry{}, nil
	}
	var out []MatingEntry
	for _, table := range tables {
		for _, row := range tableBodyRows(table) {
			cells := rowCellTexts(row)
			if len(cells) == 0 {
				continue
			}
			m := MatingEntry{}
			if len(cells) > 0 {
				m.Year = cells[0]
			}
			if len(cells) > 1 {
				m.SireName = cells[1]
			}
			if len(cells) > 2 {
				m.DamName = cells[2]
			}
			if len(cells) > 3 {
				m.CoverCount = cells[3]
			}
			if len(cells) > 4 {
				m.FoalCount = cells[4]
			}
			if len(cells) > 5 {
				m.Status = cells[5]
			}
			if m.Year == "" && m.SireName == "" && m.DamName == "" {
				continue
			}
			out = append(out, m)
		}
	}
	return out, nil
}

func parseOffspring(body []byte) ([]OffspringEntry, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, permanentErr("parse TJK offspring HTML failed", 0)
	}
	tables := findTablesUnderClass(doc, "grid_24")
	if len(tables) == 0 {
		return []OffspringEntry{}, nil
	}
	var out []OffspringEntry
	for _, table := range tables {
		for _, row := range tableBodyRows(table) {
			cells := rowCellTexts(row)
			if len(cells) == 0 {
				continue
			}
			o := OffspringEntry{}
			if len(cells) > 0 {
				o.Name = cells[0]
			}
			if len(cells) > 1 {
				o.BirthYear = cells[1]
			}
			if len(cells) > 2 {
				o.SireName = cells[2]
			}
			if len(cells) > 3 {
				o.DamName = cells[3]
			}
			if len(cells) > 4 {
				o.RaceCount = cells[4]
			}
			if len(cells) > 5 {
				o.First = cells[5]
			}
			if len(cells) > 6 {
				o.Second = cells[6]
			}
			if len(cells) > 7 {
				o.Third = cells[7]
			}
			if len(cells) > 8 {
				o.Fourth = cells[8]
			}
			if len(cells) > 9 {
				o.Earning = cells[9]
			}
			if o.Name == "" {
				continue
			}
			out = append(out, o)
		}
	}
	return out, nil
}

func hasExactElementText(doc *html.Node, element, text string) bool {
	found := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode && n.Data == element && normalizedText(n) == text {
			found = true
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return found
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
