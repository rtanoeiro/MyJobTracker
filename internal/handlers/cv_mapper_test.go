package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"job-applications/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

func cvRow() db.GetCVByIDRow {
	return db.GetCVByIDRow{
		ID:                    7,
		Title:                 "Generalist",
		Profile:               []byte(`{"name":"Jane","location":"London"}`),
		Experiences:           []byte(`[{"company":"ACME","field":"Engineer","start":"2021-03","end":"2022-06","bullets":["won big","helped"]}]`),
		Education:             pgtype.Text{String: "UCL, CS, 2015", Valid: true},
		Certifications:        pgtype.Text{String: "AWS, Amazon", Valid: true},
		AcademicContributions: pgtype.Text{String: "a paper", Valid: true},
		Skills:                pgtype.Text{String: "volunteer; languages", Valid: true},
	}
}

func TestBuildCVEditFormData_DecodesAndSplits(t *testing.T) {
	data, err := buildCVEditFormData(cvRow())
	if err != nil {
		t.Fatalf("buildCVEditFormData: %v", err)
	}

	if data.Title != "Generalist" || data.ID != 7 {
		t.Fatalf("unexpected header: id=%d title=%q", data.ID, data.Title)
	}
	if data.Profile.Name != "Jane" || data.Profile.Location != "London" {
		t.Fatalf("profile not decoded: %+v", data.Profile)
	}

	if len(data.Experiences) != 1 {
		t.Fatalf("expected 1 experience, got %d", len(data.Experiences))
	}
	e := data.Experiences[0]
	if e.Index != 0 || e.Company != "ACME" || e.Field != "Engineer" {
		t.Fatalf("experience basics wrong: %+v", e)
	}
	// Stored combined dates preserved…
	if e.Start != "2021-03" || e.End != "2022-06" {
		t.Fatalf("combined dates not preserved: start=%q end=%q", e.Start, e.End)
	}
	// …and split into the month/year the template selects need.
	if e.StartYear != "2021" || e.StartMonth != "03" {
		t.Fatalf("start not split: %s-%s", e.StartYear, e.StartMonth)
	}
	if e.EndYear != "2022" || e.EndMonth != "06" {
		t.Fatalf("end not split: %s-%s", e.EndYear, e.EndMonth)
	}
	// Bullets padded to the full editor row count.
	if len(e.Bullets) != cvExperienceBulletsLimit {
		t.Fatalf("expected %d bullet slots, got %d", cvExperienceBulletsLimit, len(e.Bullets))
	}
	if e.Bullets[0] != "won big" || e.Bullets[1] != "helped" || e.Bullets[2] != "" || e.Bullets[3] != "" {
		t.Fatalf("bullets not padded/preserved: %v", e.Bullets)
	}

	if data.Education != "UCL, CS, 2015" {
		t.Fatalf("education not joined: %q", data.Education)
	}
	if data.Certifications != "AWS, Amazon" {
		t.Fatalf("certifications not joined: %q", data.Certifications)
	}
	if data.Academic != "a paper" {
		t.Fatalf("academic contributions not joined: %q", data.Academic)
	}
	if data.Skills != "volunteer; languages" {
		t.Fatalf("Skills not joined: %q", data.Skills)
	}
}

func TestBuildCVEditFormData_CurrentRoleHasNoEnd(t *testing.T) {
	row := cvRow()
	row.Experiences = []byte(`[{"company":"ACME","field":"Eng","start":"2020-01","current":true}]`)

	data, err := buildCVEditFormData(row)
	if err != nil {
		t.Fatalf("buildCVEditFormData: %v", err)
	}
	e := data.Experiences[0]
	if !e.Current {
		t.Fatal("expected current role flag")
	}
	if e.End != "" || e.EndYear != "" || e.EndMonth != "" {
		t.Fatalf("expected empty end for current role, got %q/%s/%s", e.End, e.EndYear, e.EndMonth)
	}
	if e.StartYear != "2020" || e.StartMonth != "01" {
		t.Fatalf("start not split: %s-%s", e.StartYear, e.StartMonth)
	}
}

func TestExperienceRenderFieldsNeverPersist(t *testing.T) {
	exp := cvExperience{
		Company:    "ACME",
		Start:      "2021-03",
		End:        "2022-06",
		Index:      3,
		Months:     cvMonths(),
		Years:      []string{"2022"},
		StartMonth: "03",
		StartYear:  "2021",
		EndMonth:   "06",
		EndYear:    "2022",
	}

	raw, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The stored JSON must contain exactly the persistence fields and never the
	// template-only ones (Index, Months, Years, month/year splits).
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, leaked := range []string{"index", "months", "years", "startMonth", "startYear", "endMonth", "endYear"} {
		if _, ok := out[leaked]; ok {
			t.Errorf("template-only field %q leaked into stored JSON: %s", leaked, raw)
		}
	}
	for _, kept := range []string{"company", "start", "end"} {
		if _, ok := out[kept]; !ok {
			t.Errorf("persistence field %q missing from stored JSON: %s", kept, raw)
		}
	}
}

func TestExtractIndices_DistinctSortedSet(t *testing.T) {
	// A block deleted in the middle leaves gaps (here index 1 is gone). Each
	// block spans several keys, but every distinct present index must be
	// reported exactly once, in ascending order.
	form := url.Values{}
	form.Set("experiences_0_company", "ACME")
	form.Set("experiences_0_field", "Engineer")
	form.Set("experiences_0_start_year", "2021")
	form.Set("experiences_0_bullet_0", "won big")
	form.Set("experiences_2_company", "Globex")
	form.Set("experiences_2_current", "true")
	form.Set("unrelated_field", "ignore me")

	got := extractIndices(form, "experiences_")
	want := []int32{0, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractIndices = %v, want %v", got, want)
	}
}

func TestExtractIndices_TwoDigitAndNonNumericKeys(t *testing.T) {
	form := url.Values{}
	form.Set("academic_3", "a paper")
	form.Set("academic_10", "a talk")
	form.Set("academic_foo", "not an index")

	got := extractIndices(form, "academic_")
	want := []int32{3, 10}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractIndices = %v, want %v", got, want)
	}
}

func TestBuildCreateCVParams_ParsesForm(t *testing.T) {
	form := url.Values{}
	form.Set("title", "Generalist")
	form.Set("profile_name", "Jane Doe")
	form.Set("profile_location", "London")

	// Experience index 1 was deleted mid-list; only 0 and 2 remain.
	form.Set("experiences_0_company", "ACME")
	form.Set("experiences_0_field", "Engineer")
	form.Set("experiences_0_start_month", "03")
	form.Set("experiences_0_start_year", "2021")
	form.Set("experiences_0_end_month", "06")
	form.Set("experiences_0_end_year", "2022")
	form.Set("experiences_0_bullet_0", "won big")
	form.Set("experiences_0_bullet_1", "helped")
	form.Set("experiences_2_company", "Globex")
	form.Set("experiences_2_field", "Product Manager")
	form.Set("experiences_2_start_month", "01")
	form.Set("experiences_2_start_year", "2022")
	form.Set("experiences_2_current", "true")
	form.Set("experiences_2_bullet_0", "shipped")

	form.Set("education", "UCL, Computer Science, 2015")
	form.Set("certifications", "AWS Certified, Amazon")
	form.Set("academic", "a paper;a talk")
	form.Set("skills", "volunteering;languages")

	params, err := buildCreateCVParams(formRequest(http.MethodPost, "/cvs", form), 7)
	if err != nil {
		t.Fatalf("buildCreateCVParams: %v", err)
	}
	if params.UserID != 7 || params.Title != "Generalist" {
		t.Fatalf("header wrong: userID=%d title=%q", params.UserID, params.Title)
	}

	var profile cvProfile
	if err := json.Unmarshal(params.Profile, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profile.Name != "Jane Doe" || profile.Location != "London" {
		t.Fatalf("profile wrong: %+v", profile)
	}

	var experiences []cvExperience
	if err := json.Unmarshal(params.Experiences, &experiences); err != nil {
		t.Fatalf("decode experiences: %v", err)
	}
	if len(experiences) != 2 {
		t.Fatalf("expected 2 experiences, got %d: %+v", len(experiences), experiences)
	}
	if experiences[0].Company != "ACME" || experiences[0].Field != "Engineer" {
		t.Fatalf("first experience wrong: %+v", experiences[0])
	}
	if experiences[0].Start != "2021-03" || experiences[0].End != "2022-06" {
		t.Fatalf("first experience dates wrong: %q-%q", experiences[0].Start, experiences[0].End)
	}
	if !reflect.DeepEqual(experiences[0].Bullets, []string{"won big", "helped"}) {
		t.Fatalf("first experience bullets wrong: %v", experiences[0].Bullets)
	}
	if experiences[1].Company != "Globex" || !experiences[1].Current {
		t.Fatalf("second experience wrong: %+v", experiences[1])
	}
	if experiences[1].Start != "2022-01" || experiences[1].End != "" {
		t.Fatalf("current role should keep start and drop end: %q-%q", experiences[1].Start, experiences[1].End)
	}

	wantText := map[string]pgtype.Text{
		"education":      {String: "UCL, Computer Science, 2015", Valid: true},
		"certifications": {String: "AWS Certified, Amazon", Valid: true},
		"academic":       {String: "a paper;a talk", Valid: true},
		"skills":         {String: "volunteering;languages", Valid: true},
	}
	gotText := map[string]pgtype.Text{
		"education":      params.Education,
		"certifications": params.Certifications,
		"academic":       params.AcademicContributions,
		"skills":         params.Skills,
	}
	for section, want := range wantText {
		if gotText[section] != want {
			t.Fatalf("%s wrong: got %+v, want %+v", section, gotText[section], want)
		}
	}
}

func TestMarshalCVExperiences_EdgeCases(t *testing.T) {
	form := url.Values{}
	// End before start must default to start.
	form.Set("experiences_0_company", "ACME")
	form.Set("experiences_0_start_month", "06")
	form.Set("experiences_0_start_year", "2022")
	form.Set("experiences_0_end_month", "01")
	form.Set("experiences_0_end_year", "2021")
	// Current role: end is dropped entirely.
	form.Set("experiences_1_company", "Globex")
	form.Set("experiences_1_start_month", "01")
	form.Set("experiences_1_start_year", "2022")
	form.Set("experiences_1_current", "true")
	// Bullets only (no company), sparse bullet slots.
	form.Set("experiences_2_bullet_0", "led team")
	form.Set("experiences_2_bullet_2", "shipped")
	// Blank block (whitespace only) must be skipped.
	form.Set("experiences_3_company", "   ")

	raw, err := marshalCVExperiences(form)
	if err != nil {
		t.Fatalf("marshalCVExperiences: %v", err)
	}
	var experiences []cvExperience
	if err := json.Unmarshal(raw, &experiences); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(experiences) != 3 {
		t.Fatalf("expected 3 experiences, got %d: %+v", len(experiences), experiences)
	}
	if experiences[0].End != "2022-06" {
		t.Fatalf("end should default to start, got %q", experiences[0].End)
	}
	if !experiences[1].Current || experiences[1].End != "" || experiences[1].Start != "2022-01" {
		t.Fatalf("current role wrong: %+v", experiences[1])
	}
	if experiences[2].Company != "" || experiences[2].Bullets[0] != "led team" || experiences[2].Bullets[1] != "shipped" || len(experiences[2].Bullets) != 2 {
		t.Fatalf("bullets-only experience wrong: %+v", experiences[2])
	}
}

func TestPGTextFromForm_StoresVerbatim(t *testing.T) {
	form := url.Values{}
	form.Set("academic", "a paper;a talk")

	got := pgTextFromForm(form, "academic")
	if got != (pgtype.Text{String: "a paper;a talk", Valid: true}) {
		t.Fatalf("pgTextFromForm = %+v, want the field stored verbatim", got)
	}

	if blank := pgTextFromForm(form, "missing"); blank.Valid {
		t.Fatalf("absent field should map to NULL, got %+v", blank)
	}
}

func TestBuildCreateCVParams_MissingTitle(t *testing.T) {
	params, err := buildCreateCVParams(formRequest(http.MethodPost, "/cvs", url.Values{}), 1)
	if err == nil {
		t.Fatalf("expected missing title error, got nil with params %+v", params)
	}
}

func TestBuildCreateCVParams_EmptySections(t *testing.T) {
	params, err := buildCreateCVParams(formRequest(http.MethodPost, "/cvs", url.Values{"title": {"Software Engineer"}}), 1)
	if err != nil {
		t.Fatalf("buildCreateCVParams: %v", err)
	}
	for section, text := range map[string]pgtype.Text{
		"education":      params.Education,
		"certifications": params.Certifications,
		"academic":       params.AcademicContributions,
		"skills":         params.Skills,
	} {
		if text.Valid {
			t.Fatalf("%s section should be NULL when blank, got %+v", section, text)
		}
	}
}

func TestBuildCVEditFormData_PopulatesSummary(t *testing.T) {
	row := cvRow()
	row.Summary = pgtype.Text{String: "A short summary", Valid: true}

	data, err := buildCVEditFormData(row)
	if err != nil {
		t.Fatalf("buildCVEditFormData: %v", err)
	}
	if data.Summary != "A short summary" {
		t.Fatalf("summary = %q, want %q", data.Summary, "A short summary")
	}
}

func TestBuildCVEditFormData_ExperiencesDecodeError(t *testing.T) {
	row := cvRow()
	row.Experiences = []byte(`[{"company":`)

	if _, err := buildCVEditFormData(row); err == nil {
		t.Fatal("expected experiences decode error, got nil")
	}
}

func TestCleanFormFields_RemovesEmptyValues(t *testing.T) {
	form := url.Values{
		"title":  {"CV"},
		"empty":  {""},
		"spaces": {"   "},
	}

	cleaned := cleanFormFields(form)

	if cleaned.Get("title") != "CV" {
		t.Errorf("title = %q, want CV", cleaned.Get("title"))
	}
	if _, ok := cleaned["empty"]; ok {
		t.Error("expected empty-valued field to be removed")
	}
	if cleaned.Get("spaces") != "   " {
		t.Errorf("whitespace-only field should be preserved verbatim, got %q", cleaned.Get("spaces"))
	}
}
