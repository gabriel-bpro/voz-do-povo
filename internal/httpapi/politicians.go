package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"voz-do-povo/internal/sync"
)

// termStartYear is the first year of the current legislative term: the
// profile's year selector never reaches further back (history = current term).
const termStartYear = 2023

// Source names as shown to citizens (pt-BR), keyed by our internal source ids.
const (
	sourceNameRegistry = "Câmara dos Deputados — cadastro de deputados"
	sourceNameCEAP     = "Câmara dos Deputados — CEAP (cota parlamentar)"
)

// sourceMeta is the provenance block every data-carrying response includes:
// the dataset's source name and when we last synced it (null if never).
type sourceMeta struct {
	Key      string     `json:"key"`
	Name     string     `json:"name"`
	SyncedAt *time.Time `json:"syncedAt"`
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing query parameter q"})
		return
	}
	norm := sync.NormalizeName(q)

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, parliamentary_name, uf
		FROM curated.politician
		WHERE name_norm % $1
		ORDER BY similarity(name_norm, $1) DESC, parliamentary_name
		LIMIT 20`, norm)
	if err != nil {
		log.Printf("search %q: %v", q, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
		return
	}
	defer rows.Close()

	type result struct {
		ID                int64   `json:"id"`
		ParliamentaryName string  `json:"parliamentaryName"`
		UF                *string `json:"uf"`
	}
	results := []result{}
	for rows.Next() {
		var res result
		if err := rows.Scan(&res.ID, &res.ParliamentaryName, &res.UF); err != nil {
			log.Printf("search %q scan: %v", q, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
			return
		}
		results = append(results, res)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":   q,
		"results": results,
		"sources": []sourceMeta{s.registrySource(r.Context())},
	})
}

func (s *server) handleProfile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid politician id"})
		return
	}

	year := time.Now().Year()
	if raw := r.URL.Query().Get("year"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil || y < termStartYear || y > time.Now().Year() {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("year must be between %d and the current year", termStartYear),
			})
			return
		}
		year = y
	}

	ctx := r.Context()
	p, err := s.loadPolitician(ctx, id)
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "politician not found"})
		return
	}
	if err != nil {
		log.Printf("profile %d: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "profile lookup failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"politician": p,
		"summary":    profileSummary(p),
		// Provenance: identity data comes from the registry sync; the
		// expenses section carries its own source block.
		"sources":  []sourceMeta{s.registrySource(ctx)},
		"expenses": s.expenseSection(ctx, id, year),
	})
}

type politicianView struct {
	ID                int64   `json:"id"`
	CivilName         string  `json:"civilName"`
	ParliamentaryName string  `json:"parliamentaryName"`
	UF                *string `json:"uf"`
	Party             *string `json:"party"`
}

// loadPolitician reads the canonical record; party comes from the most recent
// linked expense (the registry's sigla is not stored on the politician — the
// party at expense time is the meaningful one). sql.ErrNoRows = unknown id.
func (s *server) loadPolitician(ctx context.Context, id int64) (politicianView, error) {
	var p politicianView
	p.ID = id
	err := s.db.QueryRowContext(ctx, `
		SELECT civil_name, parliamentary_name, uf
		FROM curated.politician WHERE id = $1`, id).
		Scan(&p.CivilName, &p.ParliamentaryName, &p.UF)
	if err != nil {
		return p, err
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT pa.sigla
		FROM curated.expense e JOIN curated.party pa ON pa.id = e.party_id
		WHERE e.politician_id = $1
		ORDER BY e.year DESC, e.month DESC
		LIMIT 1`, id).Scan(&p.Party); err != nil && err != sql.ErrNoRows {
		log.Printf("profile %d party lookup: %v", id, err)
	}
	return p, nil
}

// profileSummary is the templated identity sentence. Zero LLM, no free prose:
// the prototype's template, kept gender-neutral because the schema does not
// (yet) carry the registry's sexo field.
func profileSummary(p politicianView) string {
	where := ""
	switch {
	case p.Party != nil && p.UF != nil:
		where = fmt.Sprintf(" por %s (%s)", *p.UF, *p.Party)
	case p.UF != nil:
		where = fmt.Sprintf(" por %s", *p.UF)
	}
	return fmt.Sprintf("%s é deputado(a) federal%s, no mandato 2023–2026.", p.ParliamentaryName, where)
}

// Attention-badge rule 1 (issue 14): an expense item badges when its net
// amount is above the median of its bucket — house × expense category over
// the trailing 12 months — and at least R$ 1.000. Buckets with fewer than
// 30 items never badge; negative values (glosas/ressarcimentos) are excluded
// from both the median sample and the badge (the floor already bars them).
// Medians are computed on demand over the curated layer.
const (
	badgeMinAmount   = 1000.0
	badgeMinBucketN  = 30
	medianBucketsCTE = `
		WITH buckets AS (
			SELECT house, category,
			       COUNT(*) AS n,
			       percentile_cont(0.5) WITHIN GROUP (ORDER BY amount_net::float8) AS median
			FROM curated.expense
			WHERE amount_net >= 0
			  AND issued_at >= (CURRENT_DATE - INTERVAL '12 months')::date
			GROUP BY house, category
		)`
)

// expenseSection builds the profile's expenses block. When the politician has
// no Câmara link and no linked expenses, it returns the honest "not linked"
// state instead of fabricating numbers or hiding the section.
func (s *server) expenseSection(ctx context.Context, politicianID int64, year int) map[string]any {
	source := sourceMeta{Key: sync.SourceCamaraCEAP, Name: sourceNameCEAP, SyncedAt: s.lastExpenseSync(ctx)}

	var linked bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM curated.politician_source_id
			WHERE politician_id = $1 AND source = 'camara'
		) OR EXISTS (
			SELECT 1 FROM curated.expense WHERE politician_id = $1
		)`, politicianID).Scan(&linked); err != nil {
		log.Printf("profile %d link check: %v", politicianID, err)
	}
	if !linked {
		return map[string]any{
			"linked":  false,
			"message": "Ainda não conseguimos ligar os gastos desta pessoa à fonte oficial com segurança. Não mostramos números sem essa confirmação.",
			"source":  source,
		}
	}

	var total float64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_net), 0)::float8
		FROM curated.expense WHERE politician_id = $1 AND year = $2`,
		politicianID, year).Scan(&total); err != nil {
		log.Printf("profile %d total: %v", politicianID, err)
	}

	type monthTotal struct {
		Month int     `json:"month"`
		Total float64 `json:"total"`
	}
	monthly := make([]monthTotal, 12)
	for i := range monthly {
		monthly[i].Month = i + 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT month, SUM(amount_net)::float8
		FROM curated.expense WHERE politician_id = $1 AND year = $2
		GROUP BY month`, politicianID, year)
	if err != nil {
		log.Printf("profile %d monthly: %v", politicianID, err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var m int
			var t float64
			if rows.Scan(&m, &t) == nil && m >= 1 && m <= 12 {
				monthly[m-1].Total = t
			}
		}
	}

	// The badge carries the bare bucket median (never a multiplier): the
	// frontend renders the locked neutral copy from category + median.
	type badge struct {
		Median float64 `json:"median"`
	}
	type item struct {
		Category     string  `json:"category"`
		SupplierName *string `json:"supplierName"`
		IssuedAt     *string `json:"issuedAt"`
		AmountNet    float64 `json:"amountNet"`
		DocumentURL  *string `json:"documentUrl"`
		Badge        *badge  `json:"badge,omitempty"`
	}
	items := []item{}
	itemRows, err := s.db.QueryContext(ctx, medianBucketsCTE+`
		SELECT e.category, e.supplier_name, to_char(e.issued_at, 'YYYY-MM-DD'),
		       e.amount_net::float8, e.document_url,
		       CASE WHEN b.n >= $3 AND e.amount_net >= $4 AND e.amount_net > b.median
		            THEN b.median END AS badge_median
		FROM curated.expense e
		LEFT JOIN buckets b ON b.house = e.house AND b.category = e.category
		WHERE e.politician_id = $1 AND e.year = $2
		ORDER BY e.amount_net DESC
		LIMIT 10`, politicianID, year, badgeMinBucketN, badgeMinAmount)
	if err != nil {
		log.Printf("profile %d largest items: %v", politicianID, err)
	} else {
		defer itemRows.Close()
		for itemRows.Next() {
			var it item
			var badgeMedian sql.NullFloat64
			if itemRows.Scan(&it.Category, &it.SupplierName, &it.IssuedAt, &it.AmountNet, &it.DocumentURL, &badgeMedian) == nil {
				if badgeMedian.Valid {
					it.Badge = &badge{Median: badgeMedian.Float64}
				}
				items = append(items, it)
			}
		}
	}

	// Header rollup: how many of the year's expenses trip the badge rule.
	var flaggedCount int
	if err := s.db.QueryRowContext(ctx, medianBucketsCTE+`
		SELECT COUNT(*)
		FROM curated.expense e
		JOIN buckets b ON b.house = e.house AND b.category = e.category
		WHERE e.politician_id = $1 AND e.year = $2
		  AND b.n >= $3 AND e.amount_net >= $4 AND e.amount_net > b.median`,
		politicianID, year, badgeMinBucketN, badgeMinAmount).Scan(&flaggedCount); err != nil {
		log.Printf("profile %d flagged count: %v", politicianID, err)
	}

	// The selector covers the whole current term (2023+), even years without
	// a single expense — a citizen must be able to confirm "nothing spent".
	years := []int{}
	for y := termStartYear; y <= time.Now().Year(); y++ {
		years = append(years, y)
	}

	return map[string]any{
		"linked":         true,
		"year":           year,
		"availableYears": years,
		"total":          total,
		"monthly":        monthly,
		"largestItems":   items,
		"flaggedCount":   flaggedCount,
		"source":         source,
	}
}

// registrySource derives the registry dataset's freshness from the curated
// layer (the app never reads raw): every registry sync run touches
// updated_at (or created_at on first insert), so the max is the last run.
func (s *server) registrySource(ctx context.Context) sourceMeta {
	var at *time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(GREATEST(created_at, updated_at)) FROM curated.politician`).Scan(&at); err != nil {
		log.Printf("registry freshness: %v", err)
	}
	return sourceMeta{Key: sync.SourceCamaraDeputies, Name: sourceNameRegistry, SyncedAt: at}
}

// lastExpenseSync derives CEAP's last sync from curated rows: every sync run
// bumps synced_at (upsert), so MAX is the last run that saw data. nil when
// never synced.
func (s *server) lastExpenseSync(ctx context.Context) *time.Time {
	var at *time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(synced_at) FROM curated.expense WHERE house = 'camara'`).Scan(&at); err != nil {
		log.Printf("ceap freshness: %v", err)
	}
	return at
}
