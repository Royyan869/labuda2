package geography

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	lvlProvince = "province"
	lvlRegency  = "regency"
	lvlDistrict = "district"
	lvlVillage  = "village"
)

// seedRows decompresses and parses the embedded master seed.
func seedRows(t *testing.T) []struct{ code, level, name, parent, postal string } {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(seedGz))
	require.NoError(t, err)
	defer gz.Close()

	type row = struct{ code, level, name, parent, postal string }
	var rows []row
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		require.Len(t, f, 5)
		rows = append(rows, row{f[0], f[1], f[2], f[3], f[4]})
	}
	require.NoError(t, scanner.Err())
	return rows
}

// TestEmbeddedSeed_ExactCompleteness proves the master holds the exact,
// verified-complete Indonesian hierarchy — no records silently dropped.
func TestEmbeddedSeed_ExactCompleteness(t *testing.T) {
	rows := seedRows(t)

	counts := map[string]int{}
	seen := map[string]string{}
	for _, r := range rows {
		counts[r.level]++
		_, dup := seen[r.code]
		require.Falsef(t, dup, "duplicate code %s", r.code)
		seen[r.code] = r.level
	}

	require.Equal(t, 38, counts[lvlProvince], "provinces")
	require.Equal(t, 514, counts[lvlRegency], "regencies")
	require.Equal(t, 7284, counts[lvlDistrict], "districts")
	require.Equal(t, 83307, counts[lvlVillage], "villages")
	require.Equal(t, 91143, len(rows), "total rows")
}

// TestEmbeddedSeed_StructuralIntegrity proves every child has a valid parent,
// code/level lengths are correct, and the code is prefixed by its parent code.
func TestEmbeddedSeed_StructuralIntegrity(t *testing.T) {
	rows := seedRows(t)

	levelOf := map[string]string{}
	codeLen := map[string]int{lvlProvince: 2, lvlRegency: 4, lvlDistrict: 6, lvlVillage: 10}
	for _, r := range rows {
		levelOf[r.code] = r.level
		require.Equalf(t, codeLen[r.level], len(r.code), "%s %s code length", r.level, r.code)
		require.NotEmpty(t, r.name)
	}

	for _, r := range rows {
		switch r.level {
		case lvlProvince:
			require.Emptyf(t, r.parent, "province %s must have no parent", r.code)
		case lvlRegency:
			require.Equalf(t, lvlProvince, levelOf[r.parent], "regency %s parent %s", r.code, r.parent)
		case lvlDistrict:
			require.Equalf(t, lvlRegency, levelOf[r.parent], "district %s parent %s", r.code, r.parent)
		case lvlVillage:
			require.Equalf(t, lvlDistrict, levelOf[r.parent], "village %s parent %s", r.code, r.parent)
		}
		if r.parent != "" {
			require.Equalf(t, r.parent, r.code[:len(r.parent)], "child %s must be prefixed by parent %s", r.code, r.parent)
		}
	}
}

// TestEmbeddedSeed_CriticalChain proves the exact hierarchy restored by this
// fix: 12 -> 1204 -> 120428 -> 1204282001, with canonical names and postal.
func TestEmbeddedSeed_CriticalChain(t *testing.T) {
	rows := seedRows(t)
	byCode := map[string]struct{ level, name, parent, postal string }{}
	for _, r := range rows {
		byCode[r.code] = struct{ level, name, parent, postal string }{r.level, r.name, r.parent, r.postal}
	}

	require.Equal(t, "province", byCode["12"].level)
	require.Equal(t, "Sumatera Utara", byCode["12"].name)

	require.Equal(t, "regency", byCode["1204"].level)
	require.Equal(t, "Kabupaten Nias", byCode["1204"].name)
	require.Equal(t, "12", byCode["1204"].parent)

	require.Equal(t, "district", byCode["120428"].level)
	require.Equal(t, "Ma'u", byCode["120428"].name)
	require.Equal(t, "1204", byCode["120428"].parent)

	require.Equal(t, "village", byCode["1204282001"].level)
	require.Equal(t, "Balodano", byCode["1204282001"].name)
	require.Equal(t, "120428", byCode["1204282001"].parent)
	require.Equal(t, "22855", byCode["1204282001"].postal)
}

// TestEmbeddedSeed_RestoredDistricts proves every district restored by this fix
// is present with its authoritative name and parent.
func TestEmbeddedSeed_RestoredDistricts(t *testing.T) {
	rows := seedRows(t)
	byCode := map[string]string{}
	for _, r := range rows {
		if r.level == lvlDistrict {
			byCode[r.code] = r.name + "|" + r.parent
		}
	}
	want := map[string]string{
		"120428": "Ma'u|1204",
		"120435": "Sogae'adu|1204",
		"121421": "O'o'u|1214",
		"121423": "Hilisalawa'ahe|1214",
		"121425": "Sidua'ori|1214",
		"122504": "Moro'o|1225",
		"122508": "Ulu Moro'o|1225",
		"127805": "Gunungsitoli Alo'oa|1278",
		"352922": "Ra'as|3529",
		"520503": "Hu'u|5205",
		"530210": "KI'E|5302",
		"710410": "Tampan' Amma|7104",
		"731010": "Minasate'ne|7310",
		"731813": "Sangalla'|7318",
		"731833": "Sangalla' Selatan|7318",
		"731834": "Sangalla' Utara|7318",
		"731835": "Malimbong Balepe'|7318",
		"732612": "Dende' Piongan Napo|7326",
		"940516": "Mage'abume|9405",
	}
	require.Len(t, want, 19)
	for code, expected := range want {
		require.Equalf(t, expected, byCode[code], "restored district %s", code)
	}
}
