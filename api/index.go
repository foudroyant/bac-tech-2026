package handler

import (
	"encoding/json"
	"fmt"
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

var cachedStudents []Student
var cachedOpts struct {
	Series       []string
	Mentions     []string
	Departements []string
}

func init() {
	data, err := os.ReadFile("database.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, &cachedStudents); err != nil {
		panic(err)
	}

	seen := map[string]map[string]struct{}{
		"serie": {}, "mention": {}, "dept": {},
	}
	for _, s := range cachedStudents {
		if s.Serie != "" {
			seen["serie"][s.Serie] = struct{}{}
		}
		if s.Mention != "" {
			seen["mention"][s.Mention] = struct{}{}
		}
		if s.Departement != "" {
			seen["dept"][s.Departement] = struct{}{}
		}
	}
	for k := range seen["serie"] {
		cachedOpts.Series = append(cachedOpts.Series, k)
	}
	for k := range seen["mention"] {
		cachedOpts.Mentions = append(cachedOpts.Mentions, k)
	}
	for k := range seen["dept"] {
		cachedOpts.Departements = append(cachedOpts.Departements, k)
	}
	sort.Strings(cachedOpts.Series)
	sort.Strings(cachedOpts.Mentions)
	sort.Strings(cachedOpts.Departements)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/search" {
		handleSearch(w, r)
		return
	}
	handleIndex(w, r)
}

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

func handleIndex(w http.ResponseWriter, r *http.Request) {
	perPage := 50
	total := len(cachedStudents)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	pageItems := cachedStudents[0:min(perPage, total)]

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	renderPage(w, pageItems, total, totalPages, perPage)
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
	for _, s := range cachedStudents {
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

	if total == 0 {
		fmt.Fprint(w, `<tr><td colspan="10" class="px-4 py-12 text-center text-gray-400">Aucun résultat trouvé</td></tr>`)
	} else {
		for _, s := range filtered[start:end] {
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

	fmt.Fprint(w, `<div id="pagination" hx-swap-oob="true" class="flex flex-col items-center gap-3 py-4 border-t border-gray-100">`)
	if totalPages > 1 {
		fmt.Fprint(w, `<div class="flex items-center gap-1 flex-wrap justify-center">`)
		if page > 1 {
			url := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", q, serie, mention, dept, page-1)
			fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-body" hx-swap="innerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">Précédent</button>`, url)
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
				url := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", q, serie, mention, dept, p)
				fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-body" hx-swap="innerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">%d</button>`, url, p)
			}
		}
		if page < totalPages {
			url := fmt.Sprintf("/search?q=%s&serie=%s&mention=%s&dept=%s&page=%d", q, serie, mention, dept, page+1)
			fmt.Fprintf(w, `<button hx-get="%s" hx-target="#results-body" hx-swap="innerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">Suivant</button>`, url)
		} else {
			fmt.Fprint(w, `<button disabled class="px-3 py-1.5 rounded-lg border border-gray-200 text-sm text-gray-400 cursor-not-allowed">Suivant</button>`)
		}
		fmt.Fprint(w, `</div>`)
	}
	fmt.Fprintf(w, `<div class="text-sm text-gray-500">%d résultat(s) — Page %d/%d</div>`, total, page, totalPages)
	fmt.Fprint(w, `</div>`)
}

func renderPage(w http.ResponseWriter, students []Student, total, totalPages, perPage int) {
	opts := cachedOpts

	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Résultats Bac Technique 2026</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://unpkg.com/htmx.org@1.9.12"></script>
</head>
<body class="bg-gray-50 min-h-screen">
    <div class="max-w-7xl mx-auto px-4 py-8">
        <header class="mb-8">
            <h1 class="text-3xl font-bold text-gray-900">Résultats du Bac Technique 2026</h1>
            <p class="text-gray-500 mt-1">`, total, ` candidats admis</p>
        </header>

        <form id="search-form"
              hx-get="/search"
              hx-target="#results-body"
              hx-swap="innerHTML"
              class="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-6">
            <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div class="md:col-span-4">
                    <input type="text" name="q" placeholder="Rechercher par nom, prénom, matricule, établissement..."
                           class="w-full px-4 py-2.5 rounded-lg border border-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-sm">
                </div>
                <div>
                    <label class="block text-xs font-medium text-gray-500 mb-1">Série</label>
                    <select name="serie" onchange="this.form.requestSubmit()"
                            class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-sm bg-white">
                        <option value="">Toutes</option>`)
	for _, v := range opts.Series {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, v, v)
	}
	fmt.Fprint(w, `</select>
                </div>
                <div>
                    <label class="block text-xs font-medium text-gray-500 mb-1">Mention</label>
                    <select name="mention" onchange="this.form.requestSubmit()"
                            class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-sm bg-white">
                        <option value="">Toutes</option>`)
	for _, v := range opts.Mentions {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, v, v)
	}
	fmt.Fprint(w, `</select>
                </div>
                <div>
                    <label class="block text-xs font-medium text-gray-500 mb-1">Département</label>
                    <select name="dept" onchange="this.form.requestSubmit()"
                            class="w-full px-3 py-2 rounded-lg border border-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none text-sm bg-white">
                        <option value="">Tous</option>`)
	for _, v := range opts.Departements {
		fmt.Fprintf(w, `<option value="%s">%s</option>`, v, v)
	}
	fmt.Fprint(w, `</select>
                </div>
                <div class="flex items-end gap-2">
                    <button type="submit"
                            class="px-6 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors">
                        Rechercher
                    </button>
                    <button type="reset" onclick="setTimeout(()=>this.form.requestSubmit(),10)"
                            class="px-4 py-2.5 border border-gray-300 text-sm font-medium rounded-lg hover:bg-gray-50 transition-colors">
                        Effacer
                    </button>
                </div>
            </div>
        </form>

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
	for _, s := range students {
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
	fmt.Fprint(w, `</tbody>
                </table>
            </div>

            <div id="pagination" class="flex flex-col items-center gap-3 py-4 border-t border-gray-100">`)
	if totalPages > 1 {
		fmt.Fprint(w, `<div class="flex items-center gap-1 flex-wrap justify-center">
                    <button disabled class="px-3 py-1.5 rounded-lg border border-gray-200 text-sm text-gray-400 cursor-not-allowed">Précédent</button>`)
		maxBtns := min(5, totalPages)
		for p := 1; p <= maxBtns; p++ {
			if p == 1 {
				fmt.Fprintf(w, `<span class="px-3 py-1.5 rounded-lg bg-blue-600 text-white text-sm font-medium">%d</span>`, p)
			} else {
				fmt.Fprintf(w, `<button hx-get="/search?page=%d" hx-target="#results-body" hx-swap="innerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">%d</button>`, p, p)
			}
		}
		if totalPages > 1 {
			fmt.Fprintf(w, `<button hx-get="/search?page=2" hx-target="#results-body" hx-swap="innerHTML" class="px-3 py-1.5 rounded-lg border border-gray-300 text-sm text-gray-700 hover:bg-gray-100 transition-colors">Suivant</button>`)
		}
		fmt.Fprint(w, `</div>`)
	}
	fmt.Fprintf(w, `<div class="text-sm text-gray-500">%d résultat(s) — Page 1/%d</div>`, total, totalPages)
	fmt.Fprint(w, `</div>
        </div>
    </div>

    <script>
        document.getElementById('search-form').addEventListener('reset', function(e) {
            setTimeout(() => this.requestSubmit(), 10);
        });
    </script>
</body>
</html>`)
}
