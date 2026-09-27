package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"voz-do-povo/internal/sync"
)

// termStartYear is the first year of the current legislative term: the
// profile's year selector never reaches further back (history = current term).
const termStartYear = 2023

// pixBadgeMinShare is the rule-3 threshold: the emendas section badges when
// at least this share of the current calendar year's empenhado is emenda PIX.
const pixBadgeMinShare = 0.5

// emendaBadge is the rule-3 attention badge payload on the emendas section.
type emendaBadge struct {
	PixPercent int `json:"pixPercent"`
	Year       int `json:"year"`
}

// Source names as shown to citizens (pt-BR), keyed by our internal source ids.
const (
	sourceNameRegistryCamara = "Câmara dos Deputados — cadastro de deputados"
	sourceNameRegistrySenado = "Senado Federal — cadastro de senadores"
	sourceNameCEAP           = "Câmara dos Deputados — CEAP (cota parlamentar)"
	sourceNameCEAPS          = "Senado Federal — CEAPS (cota de exercício da atividade parlamentar)"
	sourceNameEmendas        = "Portal da Transparência — emendas parlamentares"
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
		"sources": []sourceMeta{
			s.registrySource(r.Context(), sync.SourceCamaraDeputies),
			s.registrySource(r.Context(), sync.SourceSenadoSenators),
		},
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

	house := s.politicianHouse(ctx, id)
	registryKey := sync.SourceCamaraDeputies
	if house == "senado" {
		registryKey = sync.SourceSenadoSenators
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"politician": p,
		"summary":    profileSummary(p, house),
		// Provenance: identity data comes from the registry sync; the
		// expenses section carries its own source block.
		"sources":  []sourceMeta{s.registrySource(ctx, registryKey)},
		"expenses": s.expenseSection(ctx, id, year, house),
		"emendas":  s.emendaSection(ctx, id, year),
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

// politicianHouse resolves the politician's active house from the source-id
// map: "senado" wins over "camara" (a canonical person with both is a
// current senator with a deputy past). "" when no registry link exists.
func (s *server) politicianHouse(ctx context.Context, id int64) string {
	house := ""
	rows, err := s.db.QueryContext(ctx,
		`SELECT source FROM curated.politician_source_id WHERE politician_id = $1`, id)
	if err != nil {
		log.Printf("profile %d house lookup: %v", id, err)
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var src string
		if err := rows.Scan(&src); err != nil {
			log.Printf("profile %d house scan: %v", id, err)
			return house
		}
		if src == "senado" {
			house = "senado"
		} else if src == "camara" && house == "" {
			house = "camara"
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("profile %d house rows: %v", id, err)
	}
	return house
}

// profileSummary is the templated identity sentence. Zero LLM, no free prose:
// the prototype's template, kept gender-neutral because the schema does not
// (yet) carry the registry's sexo field. Senator mandates vary within a
// legislature (1/3 + 2/3 renewal), so the senador(a) template carries no
// mandate years.
func profileSummary(p politicianView, house string) string {
	where := ""
	switch {
	case p.Party != nil && p.UF != nil:
		where = fmt.Sprintf(" por %s (%s)", *p.UF, *p.Party)
	case p.UF != nil:
		where = fmt.Sprintf(" por %s", *p.UF)
	}
	if house == "senado" {
		return fmt.Sprintf("%s é senador(a) da República%s.", p.ParliamentaryName, where)
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

// expenseSection builds the profile's expenses block. When the politician
// has no registry link (either house) and no linked expenses, it returns the
// honest "not linked" state instead of fabricating numbers or hiding the
// section. house (from politicianHouse) scopes every aggregate and the
// source block — CEAP and CEAPS quotas are never mixed; "" (no registry
// link) leaves the queries unscoped.
func (s *server) expenseSection(ctx context.Context, politicianID int64, year int, house string) map[string]any {
	source := sourceMeta{
		Key:      sync.SourceCamaraCEAP,
		Name:     sourceNameCEAP,
		SyncedAt: s.lastExpenseSync(ctx, "camara"),
	}
	if house == "senado" {
		source = sourceMeta{
			Key:      sync.SourceSenadoCEAPS,
			Name:     sourceNameCEAPS,
			SyncedAt: s.lastExpenseSync(ctx, "senado"),
		}
	}

	var linked bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM curated.politician_source_id
			WHERE politician_id = $1 AND source IN ('camara', 'senado')
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
		FROM curated.expense
		WHERE politician_id = $1 AND year = $2 AND ($3 = '' OR house = $3)`,
		politicianID, year, house).Scan(&total); err != nil {
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
		FROM curated.expense
		WHERE politician_id = $1 AND year = $2 AND ($3 = '' OR house = $3)
		GROUP BY month`, politicianID, year, house)
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
		       CASE WHEN b.n >= $4 AND e.amount_net >= $5 AND e.amount_net > b.median
		            THEN b.median END AS badge_median
		FROM curated.expense e
		LEFT JOIN buckets b ON b.house = e.house AND b.category = e.category
		WHERE e.politician_id = $1 AND e.year = $2 AND ($3 = '' OR e.house = $3)
		ORDER BY e.amount_net DESC
		LIMIT 10`, politicianID, year, house, badgeMinBucketN, badgeMinAmount)
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
		WHERE e.politician_id = $1 AND e.year = $2 AND ($3 = '' OR e.house = $3)
		  AND b.n >= $4 AND e.amount_net >= $5 AND e.amount_net > b.median`,
		politicianID, year, house, badgeMinBucketN, badgeMinAmount).Scan(&flaggedCount); err != nil {
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

// registrySource derives a registry dataset's freshness from the curated
// layer (the app never reads raw): every registry sync run touches
// updated_at (or created_at on first insert) of that source's politicians,
// so the max is the last run. The source key selects the house's registry
// (sync.SourceCamaraDeputies | sync.SourceSenadoSenators).
func (s *server) registrySource(ctx context.Context, sourceKey string) sourceMeta {
	source := "camara"
	name := sourceNameRegistryCamara
	if sourceKey == sync.SourceSenadoSenators {
		source = "senado"
		name = sourceNameRegistrySenado
	}
	var at *time.Time
	if err := s.db.QueryRowContext(ctx, `
		SELECT MAX(GREATEST(p.created_at, p.updated_at))
		FROM curated.politician p
		JOIN curated.politician_source_id si ON si.politician_id = p.id
		WHERE si.source = $1`, source).Scan(&at); err != nil {
		log.Printf("registry freshness (%s): %v", source, err)
	}
	return sourceMeta{Key: sourceKey, Name: name, SyncedAt: at}
}

// emendaSection builds the profile's emendas block: yearly empenhado/pago
// totals (whole term), the selected year's breakdowns by policy area and by
// localidade (state/municipality), and the year's 5 largest emendas with the
// PIX type marker. A politician with no linked emendas gets the honest
// "not linked" state — the source gives only the author's name, so a missing
// link and "authored nothing" are indistinguishable without review.
func (s *server) emendaSection(ctx context.Context, politicianID int64, year int) map[string]any {
	source := sourceMeta{
		Key:      sync.SourceTransparenciaEmendas,
		Name:     sourceNameEmendas,
		SyncedAt: s.lastEmendaSync(ctx),
	}

	var linked bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM curated.emenda WHERE politician_id = $1)`,
		politicianID).Scan(&linked); err != nil {
		log.Printf("profile %d emenda link check: %v", politicianID, err)
	}
	if !linked {
		return map[string]any{
			"linked":  false,
			"message": "Ainda não conseguimos ligar as emendas desta pessoa à fonte oficial com segurança. Não mostramos números sem essa confirmação.",
			"source":  source,
		}
	}

	type yearTotal struct {
		Year      int     `json:"year"`
		Empenhado float64 `json:"empenhado"`
		Pago      float64 `json:"pago"`
	}
	years := []yearTotal{}
	yearRows, err := s.db.QueryContext(ctx, `
		SELECT year, SUM(amount_empenhado)::float8, SUM(amount_pago)::float8
		FROM curated.emenda
		WHERE politician_id = $1
		GROUP BY year
		ORDER BY year`, politicianID)
	if err != nil {
		log.Printf("profile %d emenda years: %v", politicianID, err)
	} else {
		defer yearRows.Close()
		for yearRows.Next() {
			var yt yearTotal
			if yearRows.Scan(&yt.Year, &yt.Empenhado, &yt.Pago) == nil {
				years = append(years, yt)
			}
		}
	}

	type breakdownRow struct {
		Label string  `json:"label"`
		Total float64 `json:"total"`
	}
	breakdown := func(column string) []breakdownRow {
		out := []breakdownRow{}
		rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
			SELECT COALESCE(%s, 'Não informado'), SUM(amount_empenhado)::float8
			FROM curated.emenda
			WHERE politician_id = $1 AND year = $2
			GROUP BY 1
			ORDER BY 2 DESC
			LIMIT 8`, column), politicianID, year)
		if err != nil {
			log.Printf("profile %d emenda breakdown %s: %v", politicianID, column, err)
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var b breakdownRow
			if rows.Scan(&b.Label, &b.Total) == nil {
				out = append(out, b)
			}
		}
		return out
	}

	type topEmenda struct {
		Code       string  `json:"code"`
		Year       int     `json:"year"`
		Tipo       string  `json:"tipo"`
		IsPIX      bool    `json:"isPix"`
		Localidade *string `json:"localidade"`
		Funcao     *string `json:"funcao"`
		Empenhado  float64 `json:"empenhado"`
		Pago       float64 `json:"pago"`
	}
	top := []topEmenda{}
	topRows, err := s.db.QueryContext(ctx, `
		SELECT code, year, tipo, is_pix, localidade, funcao,
		       amount_empenhado::float8, amount_pago::float8
		FROM curated.emenda
		WHERE politician_id = $1 AND year = $2
		ORDER BY amount_empenhado DESC
		LIMIT 5`, politicianID, year)
	if err != nil {
		log.Printf("profile %d top emendas: %v", politicianID, err)
	} else {
		defer topRows.Close()
		for topRows.Next() {
			var te topEmenda
			if topRows.Scan(&te.Code, &te.Year, &te.Tipo, &te.IsPIX,
				&te.Localidade, &te.Funcao, &te.Empenhado, &te.Pago) == nil {
				top = append(top, te)
			}
		}
	}

	return map[string]any{
		"linked":       true,
		"year":         year,
		"years":        years,
		"byArea":       breakdown("funcao"),
		"byLocalidade": breakdown("localidade"),
		"top":          top,
		"badge":        s.emendaPixBadge(ctx, politicianID),
		"source":       source,
	}
}

// emendaPixBadge trips rule 3: >= 50% of the politician's current-calendar-year
// empenhado is emenda PIX. The badge reports the actual (rounded) percentage
// and is always computed on the current year, not the ?year= the section is
// displaying. Zero current-year empenhado never badges.
func (s *server) emendaPixBadge(ctx context.Context, politicianID int64) *emendaBadge {
	currentYear := time.Now().Year()
	var pix, total float64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_empenhado) FILTER (WHERE is_pix), 0)::float8,
		       COALESCE(SUM(amount_empenhado), 0)::float8
		FROM curated.emenda
		WHERE politician_id = $1 AND year = $2`, politicianID, currentYear).
		Scan(&pix, &total); err != nil {
		log.Printf("profile %d emenda pix badge: %v", politicianID, err)
		return nil
	}
	if total <= 0 || pix/total < pixBadgeMinShare {
		return nil
	}
	return &emendaBadge{
		PixPercent: int(math.Round(100 * pix / total)),
		Year:       currentYear,
	}
}

// lastEmendaSync derives the emendas freshness from curated rows: every sync
// run bumps synced_at (upsert), so MAX is the last run that saw data. nil
// when never synced.
func (s *server) lastEmendaSync(ctx context.Context) *time.Time {
	var at *time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(synced_at) FROM curated.emenda`).Scan(&at); err != nil {
		log.Printf("emenda freshness: %v", err)
	}
	return at
}

// lastExpenseSync derives a house's last expense sync from curated rows:
// every sync run bumps synced_at (upsert), so MAX is the last run that saw
// data. nil when never synced.
func (s *server) lastExpenseSync(ctx context.Context, house string) *time.Time {
	var at *time.Time
	if err := s.db.QueryRowContext(ctx,
		`SELECT MAX(synced_at) FROM curated.expense WHERE house = $1`, house).Scan(&at); err != nil {
		log.Printf("expense freshness (%s): %v", house, err)
	}
	return at
}
