package camara

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Votes & attendance bulk files (ticket 19), all under
// dadosabertos.camara.leg.br/arquivos/<dataset>/json/<dataset>-{ano}.json —
// plain JSON in the {"dados":[...]} envelope (no zip, unlike CEAP), updated
// daily upstream. Shapes verified against the live 2026 files (2026-09-28):
//
//   - votacoes-{ano}: one row per vote (nominal AND symbolic), with siglaOrgao
//     (PLEN = plenary), idEvento (the plenary session), aprovacao (1/0,
//     sometimes absent) and descricao.
//   - votacoesVotos-{ano}: one row per deputy per NOMINAL vote (symbolic votes
//     have sparse/no records), with the deputy's API id, party sigla and UF at
//     vote time.
//   - votacoesOrientacoes-{ano}: one row per bancada per vote; siglaBancada is
//     a party sigla OR a bloc name (Governo/Oposição/Minoria); orientacao may
//     be empty (no orientation recorded).
//   - eventosPresencaDeputados-{ano}: presence per event per deputy —
//     presences only, no absences.

// arquivosBulkURL is a fmt.Sprintf pattern taking (dataset, year).
const arquivosBulkURL = "https://dadosabertos.camara.leg.br/arquivos/%s/json/%s-%d.json"

// VoteEvent is one row of votacoes-{ano}.
type VoteEvent struct {
	ID          string // e.g. "1228863-102"
	Date        string // YYYY-MM-DD
	SiglaOrgao  string // PLEN = plenary
	EventID     int64  // plenary session id (0 = none)
	Approved    *bool  // aprovacao: 1 → true, 0 → false, absent → nil
	Description string
	Raw         json.RawMessage `json:"-"`
}

// VoteCast is one row of votacoesVotos-{ano}: one deputy's vote in one
// nominal vote.
type VoteCast struct {
	VoteID     string
	VotedAt    string // "2026-06-10T14:46:15"
	DeputyID   int64  // the API id — matches the registry's source id
	PartySigla string
	UF         string
	Vote       string          // Sim / Não / Abstenção / Obstrução / ...
	Raw        json.RawMessage `json:"-"`
}

// VoteOrientation is one row of votacoesOrientacoes-{ano}.
type VoteOrientation struct {
	VoteID      string
	Bancada     string          // party sigla or bloc name (Governo/Oposição/Minoria)
	Orientation string          // Sim / Não / Liberado / "" (none recorded)
	Raw         json.RawMessage `json:"-"`
}

// PresenceRow is one row of eventosPresencaDeputados-{ano}: a deputy present
// at an event. The source publishes presences only.
type PresenceRow struct {
	EventID   int64
	StartedAt string // "2026-02-24T14:43:23"
	DeputyID  int64
	Raw       json.RawMessage `json:"-"`
}

// fetchDados GETs an arquivos bulk file and walks its "dados" array,
// streaming row by row (the votos file is ~30 MB).
func (c *Client) fetchDados(ctx context.Context, url string, each func(json.RawMessage) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("decode bulk envelope: %w", err)
		}
		if key, ok := tok.(string); ok && key == bulkPayloadKey {
			break
		}
	}
	if _, err := dec.Token(); err != nil { // opening '['
		return fmt.Errorf("decode bulk array start: %w", err)
	}
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("decode bulk row: %w", err)
		}
		if err := each(raw); err != nil {
			return err
		}
	}
	return nil
}

// bulkVoteEvent is one row of votacoes-{ano}.json.
type bulkVoteEvent struct {
	ID          string `json:"id"`
	Date        string `json:"data"`
	SiglaOrgao  string `json:"siglaOrgao"`
	EventID     int64  `json:"idEvento"`
	Aprovacao   *int   `json:"aprovacao"`
	Description string `json:"descricao"`
}

// FetchVoteEvents downloads votacoes-{ano}.json (all votes, nominal and
// symbolic).
func (c *Client) FetchVoteEvents(ctx context.Context, year int) ([]VoteEvent, error) {
	var out []VoteEvent
	err := c.fetchDados(ctx, fmt.Sprintf(arquivosBulkURL, "votacoes", "votacoes", year), func(raw json.RawMessage) error {
		var row bulkVoteEvent
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("parse votacao row: %w", err)
		}
		ev := VoteEvent{
			ID:          row.ID,
			Date:        dateOnly(row.Date),
			SiglaOrgao:  row.SiglaOrgao,
			EventID:     row.EventID,
			Description: row.Description,
			Raw:         raw,
		}
		if row.Aprovacao != nil {
			v := *row.Aprovacao == 1
			ev.Approved = &v
		}
		out = append(out, ev)
		return nil
	})
	return out, err
}

// bulkVoteCast is one row of votacoesVotos-{ano}.json. The deputy block is
// keyed "deputado_" (sic) and carries the API id as a string.
type bulkVoteCast struct {
	VoteID  string `json:"idVotacao"`
	VotedAt string `json:"dataHoraVoto"`
	Vote    string `json:"voto"`
	Deputy  struct {
		ID         string `json:"id"`
		PartySigla string `json:"siglaPartido"`
		UF         string `json:"siglaUf"`
	} `json:"deputado_"`
}

// FetchVoteCasts downloads votacoesVotos-{ano}.json (per-deputy nominal
// votes).
func (c *Client) FetchVoteCasts(ctx context.Context, year int) ([]VoteCast, error) {
	var out []VoteCast
	err := c.fetchDados(ctx, fmt.Sprintf(arquivosBulkURL, "votacoesVotos", "votacoesVotos", year), func(raw json.RawMessage) error {
		var row bulkVoteCast
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("parse voto row: %w", err)
		}
		deputyID, err := strconv.ParseInt(row.Deputy.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("parse deputy id %q in voto row: %w", row.Deputy.ID, err)
		}
		out = append(out, VoteCast{
			VoteID:     row.VoteID,
			VotedAt:    row.VotedAt,
			DeputyID:   deputyID,
			PartySigla: row.Deputy.PartySigla,
			UF:         row.Deputy.UF,
			Vote:       row.Vote,
			Raw:        raw,
		})
		return nil
	})
	return out, err
}

// bulkVoteOrientation is one row of votacoesOrientacoes-{ano}.json.
type bulkVoteOrientation struct {
	VoteID      string `json:"idVotacao"`
	Bancada     string `json:"siglaBancada"`
	Orientation string `json:"orientacao"`
}

// FetchVoteOrientations downloads votacoesOrientacoes-{ano}.json.
func (c *Client) FetchVoteOrientations(ctx context.Context, year int) ([]VoteOrientation, error) {
	var out []VoteOrientation
	err := c.fetchDados(ctx, fmt.Sprintf(arquivosBulkURL, "votacoesOrientacoes", "votacoesOrientacoes", year), func(raw json.RawMessage) error {
		var row bulkVoteOrientation
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("parse orientacao row: %w", err)
		}
		out = append(out, VoteOrientation{
			VoteID:      row.VoteID,
			Bancada:     row.Bancada,
			Orientation: row.Orientation,
			Raw:         raw,
		})
		return nil
	})
	return out, err
}

// bulkPresenceRow is one row of eventosPresencaDeputados-{ano}.json.
type bulkPresenceRow struct {
	EventID   int64  `json:"idEvento"`
	StartedAt string `json:"dataHoraInicio"`
	DeputyID  int64  `json:"idDeputado"`
}

// FetchPresence downloads eventosPresencaDeputados-{ano}.json.
func (c *Client) FetchPresence(ctx context.Context, year int) ([]PresenceRow, error) {
	var out []PresenceRow
	err := c.fetchDados(ctx, fmt.Sprintf(arquivosBulkURL, "eventosPresencaDeputados", "eventosPresencaDeputados", year), func(raw json.RawMessage) error {
		var row bulkPresenceRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("parse presenca row: %w", err)
		}
		out = append(out, PresenceRow{
			EventID:   row.EventID,
			StartedAt: row.StartedAt,
			DeputyID:  row.DeputyID,
			Raw:       raw,
		})
		return nil
	})
	return out, err
}
