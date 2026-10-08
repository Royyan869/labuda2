// Package generator builds the canonical Geography seed from the authoritative
// Indonesian source dataset (source/*.json).
//
// It is BUILD-TIME tooling only. The produced seed (data/geography.tsv.gz) is
// embedded as the master's seed input; nothing in this package is reachable at
// runtime and the source dataset is never served.
//
// INTEGRITY CONTRACT: Build NEVER silently drops a record. A child whose parent
// is absent from the source is a hard error that fails generation, so an
// incomplete source can never again produce a deceptively clean but incomplete
// master.
package generator

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Canonical level vocabulary.
const (
	LevelProvince = "province"
	LevelRegency  = "regency"
	LevelDistrict = "district"
	LevelVillage  = "village"
)

// codeLen is the required normalized (dots removed) code length per level.
var codeLen = map[string]int{
	LevelProvince: 2,
	LevelRegency:  4,
	LevelDistrict: 6,
	LevelVillage:  10,
}

// Source JSON shapes (as authored in the Indonesian dataset).
type Province struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Regency struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProvinceID string `json:"province_id"`
}

type District struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	CityID string `json:"city_id"`
}

type Village struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DistrictID string `json:"district_id"`
}

type Postal struct {
	VillageID  string `json:"villageId"`
	PostalCode string `json:"postalCode"`
}

// Source is the complete upstream dataset.
type Source struct {
	Provinces []Province
	Regencies []Regency
	Districts []District
	Villages  []Village
	Postal    []Postal
}

// Row is one canonical master row (the TSV seed line).
type Row struct {
	Code       string
	Level      string
	Name       string
	ParentCode string
	PostalCode string
}

func norm(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), ".", "") }

// ReadSource loads the source dataset from dir.
func ReadSource(dir string) (*Source, error) {
	read := func(name string, out any) error {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}
	var s Source
	if err := read("provinces.json", &s.Provinces); err != nil {
		return nil, fmt.Errorf("read provinces: %w", err)
	}
	if err := read("cities.json", &s.Regencies); err != nil {
		return nil, fmt.Errorf("read cities: %w", err)
	}
	if err := read("districts.json", &s.Districts); err != nil {
		return nil, fmt.Errorf("read districts: %w", err)
	}
	if err := read("villages.json", &s.Villages); err != nil {
		return nil, fmt.Errorf("read villages: %w", err)
	}
	if err := read("postal_codes.json", &s.Postal); err != nil {
		return nil, fmt.Errorf("read postal_codes: %w", err)
	}
	return &s, nil
}

// Build validates the source hierarchy and returns the canonical rows ordered
// province -> regency -> district -> village (by code within a level).
//
// It returns an error on any data-integrity violation: empty/duplicate code,
// wrong code length for the level, empty name, or a child whose parent is
// absent. It never drops a record.
func Build(src *Source) ([]Row, error) {
	if src == nil {
		return nil, fmt.Errorf("nil source")
	}

	levelOf := map[string]string{}
	register := func(code, level string) error {
		if code == "" {
			return fmt.Errorf("%s has an empty code", level)
		}
		if prev, dup := levelOf[code]; dup {
			return fmt.Errorf("duplicate geography code %q (levels %s and %s)", code, prev, level)
		}
		if want := codeLen[level]; len(code) != want {
			return fmt.Errorf("%s code %q has length %d, want %d", level, code, len(code), want)
		}
		levelOf[code] = level
		return nil
	}

	provinceName := map[string]string{}
	for _, p := range src.Provinces {
		code := norm(p.ID)
		if err := register(code, LevelProvince); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			return nil, fmt.Errorf("%s %s has an empty name", LevelProvince, code)
		}
		provinceName[code] = name
	}

	regencyName := map[string]string{}
	regencyParent := map[string]string{}
	for _, r := range src.Regencies {
		code := norm(r.ID)
		if err := register(code, LevelRegency); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return nil, fmt.Errorf("%s %s has an empty name", LevelRegency, code)
		}
		regencyName[code] = name
		regencyParent[code] = norm(r.ProvinceID)
	}

	districtName := map[string]string{}
	districtParent := map[string]string{}
	for _, d := range src.Districts {
		code := norm(d.ID)
		if err := register(code, LevelDistrict); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			return nil, fmt.Errorf("%s %s has an empty name", LevelDistrict, code)
		}
		districtName[code] = name
		districtParent[code] = norm(d.CityID)
	}

	villageName := map[string]string{}
	villageParent := map[string]string{}
	for _, v := range src.Villages {
		code := norm(v.ID)
		if err := register(code, LevelVillage); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(v.Name)
		if name == "" {
			return nil, fmt.Errorf("%s %s has an empty name", LevelVillage, code)
		}
		villageName[code] = name
		villageParent[code] = norm(v.DistrictID)
	}

	// Parent existence — the hard integrity gate. A missing parent FAILS
	// generation; the record is never dropped.
	for code, parent := range regencyParent {
		if _, ok := provinceName[parent]; !ok {
			return nil, fmt.Errorf("MISSING PARENT: regency %s references absent province %q", code, parent)
		}
	}
	for code, parent := range districtParent {
		if _, ok := regencyName[parent]; !ok {
			return nil, fmt.Errorf("MISSING PARENT: district %s references absent regency %q", code, parent)
		}
	}
	for code, parent := range villageParent {
		if _, ok := districtName[parent]; !ok {
			return nil, fmt.Errorf("MISSING PARENT: village %s references absent district %q", code, parent)
		}
	}

	postal := map[string]string{}
	for _, p := range src.Postal {
		vid := norm(p.VillageID)
		pc := strings.TrimSpace(p.PostalCode)
		if vid == "" || pc == "" {
			continue
		}
		if _, ok := postal[vid]; !ok {
			postal[vid] = pc
		}
	}

	rows := make([]Row, 0, len(provinceName)+len(regencyName)+len(districtName)+len(villageName))
	appendLevel := func(names, parents map[string]string, level string, withPostal bool) {
		for code, name := range names {
			row := Row{Code: code, Level: level, Name: name}
			if parents != nil {
				row.ParentCode = parents[code]
			}
			if withPostal {
				row.PostalCode = postal[code]
			}
			rows = append(rows, row)
		}
	}
	appendLevel(provinceName, nil, LevelProvince, false)
	appendLevel(regencyName, regencyParent, LevelRegency, false)
	appendLevel(districtName, districtParent, LevelDistrict, false)
	appendLevel(villageName, villageParent, LevelVillage, true)

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Level != rows[j].Level {
			return levelOrder(rows[i].Level) < levelOrder(rows[j].Level)
		}
		return rows[i].Code < rows[j].Code
	})
	return rows, nil
}

func levelOrder(level string) int {
	switch level {
	case LevelProvince:
		return 0
	case LevelRegency:
		return 1
	case LevelDistrict:
		return 2
	default:
		return 3
	}
}

// WriteGzipTSV writes rows as a gzip TSV seed (code, level, name, parent, postal).
func WriteGzipTSV(path string, rows []Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(gw)
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Code, r.Level, r.Name, r.ParentCode, r.PostalCode); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return gw.Close()
}
