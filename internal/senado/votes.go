package senado

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Votes & attendance (ticket 19). The Senado has no per-senator vote bulk
// file; two Legislative API endpoints cover it (verified live 2026-09-28):
//
//   - GET /plenario/votacao/nominal/{ano}: every plenary nominal vote of the
//     year, each with the FULL roster of senators and a per-senator code
//     (Sim/Não/Abstenção, "Votou" for secret votes, and presence codes:
//     P-NRV present-not-voted, Presidente, and the justified/absent codes
//     AP, MIS, LS, LP, LAP, NCom, NA). Attendance derives from these codes —
//     no separate presence dataset exists.
//   - GET /plenario/votacao/orientacaoBancada/{dataInicio}/{dataFim}: party
//     leadership orientations per vote, max 60 days per call — the client
//     walks the year in windows with polite pacing. Join key:
//     sequencialVotacao == the nominal feed's CodigoVotacaoSve (verified).
//
// Party identification differs per endpoint: the nominal feed uses siglas
// ("PP"), orientacaoBancada mixes siglas with full names ("Progressistas",
// "Republicanos") and blocs ("Governo"). Orientation parties are normalized
// to siglas here.

// NominalVote is one plenary nominal vote with the full senator roster.
type NominalVote struct {
	SessionCode     string // CodigoSessao — the plenary session
	SessionVoteCode string // CodigoSessaoVotacao — the vote's unique id
	VoteSeq         string // CodigoVotacaoSve — join key to orientations
	SessionDate     string // YYYY-MM-DD
	SessionStart    string // "14:00"
	Description     string
	MatterRef       string // "PDL 995/2026" ("" when not identified)
	Result          string // "A" approved / "R" rejected / "" otherwise
	Secret          bool
	Votes           []SenatorVote
	Raw             json.RawMessage `json:"-"`
}

// SenatorVote is one senator's code in one nominal vote.
type SenatorVote struct {
	SenatorCode int64
	Name        string
	PartySigla  string
	UF          string
	Vote        string
}

// VoteOrientation is one party's leadership orientation in one vote.
type VoteOrientation struct {
	VoteSeq     string          // matches NominalVote.VoteSeq
	PartySigla  string          // normalized sigla (see NormalizeOrientationParty)
	Orientation string          // normalized: Sim / Não / Abstenção / Liberado / ...
	Raw         json.RawMessage `json:"-"`
}

// nominalFeed mirrors the plenario/votacao/nominal/{ano} envelope
// (XML-heritage capitalized keys, like the senator registry).
type nominalFeed struct {
	Lista struct {
		Votacoes struct {
			Votacao []nominalVoteRow `json:"Votacao"`
		} `json:"Votacoes"`
	} `json:"ListaVotacoes"`
}

type nominalVoteRow struct {
	SessionCode     string `json:"CodigoSessao"`
	SessionVoteCode string `json:"CodigoSessaoVotacao"`
	VoteSeq         string `json:"CodigoVotacaoSve"`
	SessionDate     string `json:"DataSessao"`
	SessionStart    string `json:"HoraInicio"`
	Description     string `json:"DescricaoVotacao"`
	MatterRef       string `json:"DescricaoIdentificacaoMateria"`
	Result          string `json:"Resultado"`
	Secreta         string `json:"Secreta"`
	Votos           struct {
		VotoParlamentar []struct {
			Codigo  string `json:"CodigoParlamentar"`
			Nome    string `json:"NomeParlamentar"`
			Partido string `json:"SiglaPartido"`
			UF      string `json:"SiglaUF"`
			Voto    string `json:"Voto"`
		} `json:"VotoParlamentar"`
	} `json:"Votos"`
}

// FetchNominalVotes downloads the year's plenary nominal votes with the full
// per-senator roster.
func (c *Client) FetchNominalVotes(ctx context.Context, year int) ([]NominalVote, error) {
	var feed nominalFeed
	if err := c.get(ctx, fmt.Sprintf("%s/plenario/votacao/nominal/%d", c.legisBase, year), &feed); err != nil {
		return nil, err
	}
	out := make([]NominalVote, 0, len(feed.Lista.Votacoes.Votacao))
	for _, row := range feed.Lista.Votacoes.Votacao {
		nv := NominalVote{
			SessionCode:     row.SessionCode,
			SessionVoteCode: row.SessionVoteCode,
			VoteSeq:         row.VoteSeq,
			SessionDate:     row.SessionDate,
			SessionStart:    row.SessionStart,
			Description:     row.Description,
			MatterRef:       row.MatterRef,
			Result:          row.Result,
			Secret:          row.Secreta == "S",
		}
		for _, vp := range row.Votos.VotoParlamentar {
			code, err := strconv.ParseInt(vp.Codigo, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse senator code %q in vote %s: %w", vp.Codigo, row.SessionVoteCode, err)
			}
			nv.Votes = append(nv.Votes, SenatorVote{
				SenatorCode: code,
				Name:        vp.Nome,
				PartySigla:  vp.Partido,
				UF:          vp.UF,
				Vote:        vp.Voto,
			})
		}
		raw, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("re-marshal vote %s for raw layer: %w", row.SessionVoteCode, err)
		}
		nv.Raw = raw
		out = append(out, nv)
	}
	return out, nil
}

// orientationFeed mirrors plenario/votacao/orientacaoBancada (lowercase,
// JSON-native — the newer service).
type orientationFeed struct {
	Votacoes []struct {
		VoteSeq     int64 `json:"sequencialVotacao"`
		Orientacoes []struct {
			Party string `json:"partido"`
			Vote  string `json:"voto"`
		} `json:"orientacoesLideranca"`
	} `json:"votacoes"`
}

// orientationWindow is the endpoint's max range per call.
const orientationWindow = 59 * 24 * time.Hour

// orientationPace is the polite delay between window calls (undocumented
// rate limit — same posture as the Câmara registry detail fetches).
const orientationPace = 200 * time.Millisecond

// FetchVoteOrientations downloads the year's party leadership orientations,
// walking the endpoint in 60-day windows.
func (c *Client) FetchVoteOrientations(ctx context.Context, year int) ([]VoteOrientation, error) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	if now := time.Now(); now.Before(end) {
		end = now
	}

	var out []VoteOrientation
	for win := start; !win.After(end); win = win.Add(orientationWindow) {
		winEnd := win.Add(orientationWindow)
		if winEnd.After(end) {
			winEnd = end
		}
		url := fmt.Sprintf("%s/plenario/votacao/orientacaoBancada/%s/%s",
			c.legisBase, win.Format("20060102"), winEnd.Format("20060102"))
		var feed orientationFeed
		if err := c.get(ctx, url, &feed); err != nil {
			return nil, err
		}
		for _, v := range feed.Votacoes {
			for _, o := range v.Orientacoes {
				out = append(out, VoteOrientation{
					VoteSeq:     strconv.FormatInt(v.VoteSeq, 10),
					PartySigla:  NormalizeOrientationParty(o.Party),
					Orientation: NormalizeOrientation(o.Vote),
				})
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(orientationPace):
		}
	}
	return out, nil
}

// orientationPartyAliases maps the full party names orientacaoBancada uses
// to the siglas the rest of the pipeline (and the nominal feed) uses. Bloc
// names (Governo) are NOT parties and pass through unchanged — they never
// match a politician's party.
var orientationPartyAliases = map[string]string{
	"PROGRESSISTAS": "PP",
	"REPUBLICANOS":  "REPUBLICANOS",
	"CIDADANIA":     "CIDADANIA",
}

// NormalizeOrientationParty normalizes a party name/sigla from
// orientacaoBancada to the canonical uppercase sigla.
func NormalizeOrientationParty(p string) string {
	up := strings.ToUpper(strings.TrimSpace(p))
	if sigla, ok := orientationPartyAliases[up]; ok {
		return sigla
	}
	return up
}

// NormalizeOrientation normalizes orientation values across houses
// ("SIM"/"NAO" → "Sim"/"Não") so the API compares one vocabulary.
func NormalizeOrientation(o string) string {
	switch strings.ToUpper(strings.TrimSpace(o)) {
	case "SIM":
		return "Sim"
	case "NAO", "NÃO":
		return "Não"
	case "ABSTENÇÃO", "ABSTENCAO":
		return "Abstenção"
	case "LIBERADO":
		return "Liberado"
	case "OBSTRUÇÃO", "OBSTRUCAO":
		return "Obstrução"
	default:
		return strings.TrimSpace(o)
	}
}
