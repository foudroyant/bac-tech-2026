package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Student struct {
	Departement   string `json:"DEPARTEMENT"`
	Matricule     string `json:"MATRICULE"`
	Nom           string `json:"NOM"`
	Prenom        string `json:"PRENOM"`
	Sexe          string `json:"SEXE"`
	Date          string `json:"DATE"`
	Lieu          string `json:"LIEU"`
	Serie         string `json:"SERIE"`
	Mention       string `json:"MENTION"`
	Etablissement string `json:"ETABLISSEMENT"`
}

type FilterOpts struct {
	Series       []string
	Mentions     []string
	Departements []string
}

type PageData struct {
	Students   []Student
	Opts       FilterOpts
	Total      int
	TotalPages int
	PerPage    int
}

var (
	students []Student
	opts     FilterOpts
)

var indexTmpl *template.Template

func excelDateToStr(serialStr string) string {
	if serialStr == "" {
		return ""
	}
	serial, err := strconv.ParseFloat(serialStr, 64)
	if err != nil {
		return serialStr
	}
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, int(serial)).Format("02/01/2006")
}

func loadData() error {
	data, err := os.ReadFile("api/database.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &students); err != nil {
		return err
	}
	log.Printf("Chargé %d étudiants", len(students))

	seen := struct {
		serie, mention, dept map[string]struct{}
	}{make(map[string]struct{}), make(map[string]struct{}), make(map[string]struct{})}

	for _, s := range students {
		if s.Serie != "" {
			seen.serie[s.Serie] = struct{}{}
		}
		if s.Mention != "" {
			seen.mention[s.Mention] = struct{}{}
		}
		if s.Departement != "" {
			seen.dept[s.Departement] = struct{}{}
		}
	}
	for k := range seen.serie {
		opts.Series = append(opts.Series, k)
	}
	for k := range seen.mention {
		opts.Mentions = append(opts.Mentions, k)
	}
	for k := range seen.dept {
		opts.Departements = append(opts.Departements, k)
	}
	sort.Strings(opts.Series)
	sort.Strings(opts.Mentions)
	sort.Strings(opts.Departements)
	return nil
}

func initTemplate() {
	b, err := os.ReadFile("templates/index.html")
	if err != nil {
		log.Fatal(err)
	}
	indexTmpl = template.Must(template.New("index").Funcs(template.FuncMap{
		"excelDate": excelDateToStr,
		"seq": func(n int) []int {
			r := make([]int, n)
			for i := range r {
				r[i] = i + 1
			}
			return r
		},
	}).Parse(string(b)))
}

func main() {
	if err := loadData(); err != nil {
		log.Fatal(err)
	}
	initTemplate()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/search", handleSearch)

	addr := ":8080"
	log.Printf("Serveur démarré sur http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	perPage := 50
	total := len(students)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	pageItems := students[0:min(perPage, total)]

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	indexTmpl.Execute(w, PageData{
		Students:   pageItems,
		Opts:       opts,
		Total:      total,
		TotalPages: totalPages,
		PerPage:    perPage,
	})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("q")))
	serie := r.URL.Query().Get("serie")
	mention := r.URL.Query().Get("mention")
	dept := r.URL.Query().Get("dept")
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	perPage := 50

	var filtered []Student
	for _, s := range students {
		if q != "" {
			haystack := strings.ToUpper(s.Nom + " " + s.Prenom + " " + s.Matricule + " " + s.Etablissement)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		if serie != "" && s.Serie != serie {
			continue
		}
		if mention != "" && s.Mention != mention {
			continue
		}
		if dept != "" && s.Departement != dept {
			continue
		}
		filtered = append(filtered, s)
	}

	total := len(filtered)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	end := start + perPage
	if end > total {
		end = total
	}
	if start > end {
		start = end
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderResults(w, filtered[start:end], total, totalPages, page, q, serie, mention, dept)
}

func renderResults(w http.ResponseWriter, items []Student, total, totalPages, page int, q, serie, mention, dept string) {
	fmt.Fprint(w, `<div id="results-container">
		<div class="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
			<div class="overflow-x-auto">
				<table class="w-full">
					<thead>
						<tr class="bg-gray-50 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
							<th class="px-4 py-3">Département</th>
							<th class="px-4 py-3">Matricule</th>
							<th class="px-4 py-3">Nom</th>
							<th class="px-4 py-3">Prénom</th>
							<th class="px-4 py-3">Sexe</th>
							<th class="px-4 py-3">Date Naiss.</th>
							<th class="px-4 py-3">Lieu</th>
							<th class="px-4 py-3">Série</th>
							<th class="px-4 py-3">Mention</th>
							<th class="px-4 py-3">Établissement</th>
						</tr>
					</thead>
					<tbody id="results-body">`)

	if len(items) == 0 {
		fmt.Fprint(w, `<tr><td colspan="10" class="px-4 py-12 text-center text-gray-400">Aucun résultat trouvé</td></tr>`)
	} else {
		for _, s := range items {
			date := excelDateToStr(s.Date)
			etab := s.Etablissement
			fmt.Fprintf(w, `<tr class="hover:bg-gray-50 transition-colors even:bg-gray-50/50">
					<td class="px-4 py-3 border-b text-xs text-gray-600">%s</td>
					<td class="px-4 py-3 border-b text-xs font-mono text-gray-800">%s</td>
					<td class="px-4 py-3 border-b text-sm font-semibold text-gray-900">%s</td>
					<td class="px-4 py-3 border-b text-sm text-gray-700">%s</td>
					<td class="px-4 py-3 border-b text-xs text-gray-500">%s</td>
					<td class="px-4 py-3 border-b text-xs text-gray-600">%s</td>
					<td class="px-4 py-3 border-b text-xs text-gray-600">%s</td>
					<td class="px-4 py-3 border-b"><span class="px-2 py-0.5 rounded-full text-xs font-semibold bg-blue-50 text-blue-700 border border-blue-200">%s</span></td>
					<td class="px-4 py-3 border-b"><span class="px-2 py-0.5 rounded-full text-xs font-semibold bg-green-50 text-green-700 border border-green-200">%s</span></td>
					<td class="px-4 py-3 border-b text-xs text-gray-600 max-w-[180px] truncate" title="%s">%s</td>
				</tr>`, s.Departement, s.Matricule, s.Nom, s.Prenom, s.Sexe, date, s.Lieu, s.Serie, s.Mention, etab, etab)
		}
	}

	enc := func(v string) string {
		return urlEncode(v)
	}

	fmt.Fprint(w, `</tbody>
				</table>
			</div>
			<div id="pagination" class="flex flex-col items-center gap-3 py-4 border-t border-gray-100">`)

	if totalPages > 1 {
		fmt.Fprint(w, `<div class="flex items-center gap-1 flex-wrap justify-center">`)
		if page > 1 {
			u := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", enc(q), enc(serie), enc(mention), enc(dept), page-1)
			fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-container" hx-swap="outerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">Précédent</button>`, u)
		} else {
			fmt.Fprint(w, `<button disabled class="px-3 py-1.5 rounded-lg border border-gray-200 text-sm text-gray-400 cursor-not-allowed">Précédent</button>`)
		}
		startPage := max(1, page-2)
		endPage := min(totalPages, startPage+4)
		if endPage-startPage < 4 {
			startPage = max(1, endPage-4)
		}
		for p := startPage; p <= endPage; p++ {
			if p == page {
				fmt.Fprintf(w, `<span class="px-3 py-1.5 rounded-lg bg-blue-600 text-white text-sm font-medium">%d</span>`, p)
			} else {
				u := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", enc(q), enc(serie), enc(mention), enc(dept), p)
				fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-container" hx-swap="outerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">%d</button>`, u, p)
			}
		}
		if page < totalPages {
			u := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", enc(q), enc(serie), enc(mention), enc(dept), page+1)
			fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-container" hx-swap="outerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">Suivant</button>`, u)
		} else {
			fmt.Fprint(w, `<button disabled class="px-3 py-1.5 rounded-lg border border-gray-200 text-sm text-gray-400 cursor-not-allowed">Suivant</button>`)
		}
		fmt.Fprint(w, `</div>`)
	}
	fmt.Fprintf(w, `<div class="text-sm text-gray-500">%d résultat(s) — Page %d/%d</div>`, total, page, totalPages)
	fmt.Fprint(w, `</div>
		</div>
	</div>`)
}

func urlEncode(v string) string {
	if v == "" {
		return ""
	}
	return strings.ReplaceAll(v, " ", "%20")
}
