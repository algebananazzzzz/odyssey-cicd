package cli

import (
	"strings"
	"testing"
)

func TestRowsAndFilter(t *testing.T) {
	rows := Rows(manifest(t))
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	aws, err := Filter(rows, []string{"provider=aws"})
	if err != nil {
		t.Fatal(err)
	}
	if len(aws) != 2 {
		t.Fatalf("aws rows = %v", aws)
	}
	sub, err := Filter(rows, []string{"next"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub[0].Stack != "nextjs" {
		t.Fatalf("substring rows = %v", sub)
	}
	if _, err := Filter(rows, []string{"color=red"}); err == nil {
		t.Fatal("unknown axis accepted")
	}
}

func TestTable(t *testing.T) {
	out := Table(Rows(manifest(t)))
	if !strings.Contains(out, "STACK") || !strings.Contains(out, "node-puppeteer") {
		t.Fatalf("table:\n%s", out)
	}
}

func TestCard(t *testing.T) {
	m := manifest(t)
	rows, _ := Filter(Rows(m), []string{"nextjs"})
	out, err := Card("../..", m, []string{"dual", "single"}, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"stack          nextjs",
		"architecture   cloudflare-worker",
		"provider       cloudflare",
		"dual | single",
		"CUSTOM_DOMAIN? (per env)",
		"STATE_BUCKET",
		"CLOUDFLARE_API_TOKEN",
		"odyssey-cli new --stack nextjs",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("card missing %q:\n%s", want, out)
		}
	}
}
