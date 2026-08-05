package migration

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// SchemaMigrationsTable is the canonical Goose metadata table for Haradan.
const SchemaMigrationsTable = "hrd_schema_migrations"

var expectedTables = []string{
	"hrd_users",
	"hrd_auth_sessions",
	"hrd_one_time_credentials",
	"hrd_security_events",
	"hrd_provinces",
	"hrd_districts",
	"hrd_categories",
	"hrd_category_properties",
	"hrd_horses",
	"hrd_adverts",
	"hrd_advert_status_history",
	"hrd_favorites",
	"hrd_media_assets",
	"hrd_media_variants",
	"hrd_advert_media",
	"hrd_banners",
	"hrd_background_jobs",
	"hrd_tjk_sync_runs",
	"hrd_tjk_sync_item_errors",
	"hrd_packages",
	"hrd_advert_package_assignments",
	"hrd_advert_feature_activations",
	"hrd_campaigns",
	"hrd_notification_templates",
	"hrd_notifications",
	"hrd_user_notification_states",
}

var (
	reCreateTable     = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][\w]*)`)
	reDropTable       = regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([a-zA-Z_][\w]*)`)
	reConstraintName  = regexp.MustCompile(`(?i)CONSTRAINT\s+([a-zA-Z_][\w]*)`)
	reIndexName       = regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z_][\w]*)`)
	reFKRef           = regexp.MustCompile(`(?i)REFERENCES\s+([a-zA-Z_][\w]*)`)
	reForbiddenHR     = regexp.MustCompile(`(?i)\bhr_[a-z0-9_]*\b`)
	reCreateIFNE      = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS`)
	reDropCascade     = regexp.MustCompile(`(?i)DROP\s+[^;]*\bCASCADE\b`)
	reCreateExtension = regexp.MustCompile(`(?i)CREATE\s+EXTENSION`)
	reSearchPath      = regexp.MustCompile(`(?i)\bSET\s+search_path\b`)
	reForbiddenTables = regexp.MustCompile(`(?i)\b(payment|package|order|article|contact|ykk)s?\b`)
)

// ValidateEmbeddedMigrations scans SQL migrations before any DB operation.
func ValidateEmbeddedMigrations(fsys fs.FS) error {
	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	if len(entries) != 10 {
		return fmt.Errorf("expected 10 SQL migration files, got %d", len(entries))
	}

	created := make(map[string]struct{})
	var dropped []string

	for _, name := range entries {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		content := stripSQLCommentsAndStrings(string(raw))
		up, down, err := splitGooseSections(content)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := validateSectionSafety(name, "up", up); err != nil {
			return err
		}
		if err := validateSectionSafety(name, "down", down); err != nil {
			return err
		}

		for _, m := range reCreateTable.FindAllStringSubmatch(up, -1) {
			table := strings.ToLower(m[1])
			if table == SchemaMigrationsTable {
				return fmt.Errorf("%s: must not create %s in SQL", name, SchemaMigrationsTable)
			}
			if !strings.HasPrefix(table, "hrd_") {
				return fmt.Errorf("%s: unprefixed table create %q", name, table)
			}
			if _, exists := created[table]; exists {
				return fmt.Errorf("%s: duplicate create for %s", name, table)
			}
			created[table] = struct{}{}
		}
		for _, m := range reDropTable.FindAllStringSubmatch(down, -1) {
			table := strings.ToLower(m[1])
			if table == SchemaMigrationsTable {
				return fmt.Errorf("%s: must not drop metadata table in SQL", name)
			}
			if !strings.HasPrefix(table, "hrd_") {
				return fmt.Errorf("%s: down drops unprefixed table %q", name, table)
			}
			dropped = append(dropped, table)
		}

		for _, m := range reConstraintName.FindAllStringSubmatch(up, -1) {
			if !strings.HasPrefix(strings.ToLower(m[1]), "hrd_") {
				return fmt.Errorf("%s: constraint %q must start with hrd_", name, m[1])
			}
		}
		for _, m := range reIndexName.FindAllStringSubmatch(up, -1) {
			if !strings.HasPrefix(strings.ToLower(m[1]), "hrd_") {
				return fmt.Errorf("%s: index %q must start with hrd_", name, m[1])
			}
		}
		for _, m := range reFKRef.FindAllStringSubmatch(up, -1) {
			ref := strings.ToLower(m[1])
			if !strings.HasPrefix(ref, "hrd_") {
				return fmt.Errorf("%s: FK references unprefixed table %q", name, ref)
			}
			if strings.HasPrefix(ref, "hr_") && !strings.HasPrefix(ref, "hrd_") {
				return fmt.Errorf("%s: forbidden hr_ FK reference %q", name, ref)
			}
		}
	}

	if len(created) != 26 {
		return fmt.Errorf("expected 26 CREATE TABLE statements, got %d", len(created))
	}
	for _, table := range expectedTables {
		if _, ok := created[table]; !ok {
			return fmt.Errorf("missing expected table %s", table)
		}
	}
	for table := range created {
		found := false
		for _, exp := range expectedTables {
			if exp == table {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unexpected table %s", table)
		}
	}

	expectedDrop := map[string]struct{}{}
	for _, t := range expectedTables {
		expectedDrop[t] = struct{}{}
	}
	for _, t := range dropped {
		if _, ok := expectedDrop[t]; !ok {
			return fmt.Errorf("down drops unexpected table %s", t)
		}
		delete(expectedDrop, t)
	}
	if len(expectedDrop) != 0 {
		missing := make([]string, 0, len(expectedDrop))
		for t := range expectedDrop {
			missing = append(missing, t)
		}
		sort.Strings(missing)
		return fmt.Errorf("down missing drops for: %s", strings.Join(missing, ", "))
	}

	return nil
}

func validateSectionSafety(file, section, sql string) error {
	if reCreateIFNE.MatchString(sql) {
		return fmt.Errorf("%s %s: CREATE TABLE IF NOT EXISTS is forbidden", file, section)
	}
	if reDropCascade.MatchString(sql) {
		return fmt.Errorf("%s %s: DROP ... CASCADE is forbidden", file, section)
	}
	if reCreateExtension.MatchString(sql) {
		return fmt.Errorf("%s %s: CREATE EXTENSION is forbidden", file, section)
	}
	if reSearchPath.MatchString(sql) {
		return fmt.Errorf("%s %s: SET search_path is forbidden", file, section)
	}
	if strings.Contains(sql, "goose_db_version") {
		return fmt.Errorf("%s %s: goose_db_version must not appear in SQL", file, section)
	}
	if m := reForbiddenHR.FindString(sql); m != "" {
		// Allow hrd_*; reject bare hr_ prefix objects.
		if strings.HasPrefix(strings.ToLower(m), "hr_") && !strings.HasPrefix(strings.ToLower(m), "hrd_") {
			return fmt.Errorf("%s %s: forbidden hr_ reference %q", file, section, m)
		}
	}
	// Explicit scan for hr_ that is not hrd_
	if idx := indexForbiddenHR(sql); idx >= 0 {
		return fmt.Errorf("%s %s: forbidden hr_ reference near offset %d", file, section, idx)
	}
	if reForbiddenTables.MatchString(sql) {
		return fmt.Errorf("%s %s: forbidden payment/package/order/article/contact/YKK table reference", file, section)
	}
	return nil
}

func indexForbiddenHR(sql string) int {
	lower := strings.ToLower(sql)
	for i := 0; i < len(lower); {
		j := strings.Index(lower[i:], "hr_")
		if j < 0 {
			return -1
		}
		pos := i + j
		// skip hrd_
		if strings.HasPrefix(lower[pos:], "hrd_") {
			i = pos + 4
			continue
		}
		return pos
	}
	return -1
}

func splitGooseSections(content string) (up, down string, err error) {
	upIdx := strings.Index(content, "-- +goose Up")
	downIdx := strings.Index(content, "-- +goose Down")
	if upIdx < 0 || downIdx < 0 {
		return "", "", fmt.Errorf("missing -- +goose Up/Down markers")
	}
	if strings.Count(content, "-- +goose Up") != 1 || strings.Count(content, "-- +goose Down") != 1 {
		return "", "", fmt.Errorf("expected exactly one Up and one Down marker")
	}
	if downIdx < upIdx {
		return "", "", fmt.Errorf("Down section must follow Up section")
	}
	return content[upIdx:downIdx], content[downIdx:], nil
}

func stripSQLCommentsAndStrings(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	i := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '-' && src[i+1] == '-' {
			// Keep goose directives.
			lineEnd := strings.IndexByte(src[i:], '\n')
			if lineEnd < 0 {
				line := src[i:]
				if strings.HasPrefix(line, "-- +goose") {
					b.WriteString(line)
				}
				break
			}
			line := src[i : i+lineEnd+1]
			if strings.HasPrefix(strings.TrimSpace(line), "-- +goose") {
				b.WriteString(line)
			} else {
				b.WriteByte('\n')
			}
			i += lineEnd + 1
			continue
		}
		if src[i] == '\'' {
			b.WriteByte(' ')
			i++
			for i < len(src) {
				if src[i] == '\'' {
					if i+1 < len(src) && src[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
