package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"job-applications/internal/db"
	"job-applications/internal/utils"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TODO: Apply limits on Add Button
const (
	cvExperienceSlotsLimit   = 9
	cvExperienceBulletsLimit = 5
)

// cvForm is the view model for the create/edit forms. The four simple sections
// are rendered as one semicolon-separated text field each, so they are held as
// strings here rather than as decoded lists.
type cvForm struct {
	ID             int32
	Title          string
	Summary        string
	Profile        cvProfile
	Experiences    []cvExperience
	Education      string
	Certifications string
	Academic       string
	Skills         string
}

type cvProfile struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
}

// Some fields are ommited in JSON because they are only used in HTMX templating
type cvExperience struct {
	Company string   `json:"company,omitempty"`
	Field   string   `json:"field,omitempty"`
	Start   string   `json:"start,omitempty"`   // "YYYY-MM"; empty when unset
	End     string   `json:"end,omitempty"`     // "YYYY-MM"; empty when Current
	Current bool     `json:"current,omitempty"` // true => still employed there
	Bullets []string `json:"bullets,omitempty"`

	Index      int             `json:"-"`
	Months     []cvMonthOption `json:"-"`
	Years      []string        `json:"-"`
	StartMonth string          `json:"-"`
	StartYear  string          `json:"-"`
	EndMonth   string          `json:"-"`
	EndYear    string          `json:"-"`
}

// cvMonthOption is a struct so we can show them in order
type cvMonthOption struct {
	Value string
	Label string
}

func cvYears() []string {
	const earliestYear = 1985
	years := make([]string, 0, time.Now().Year()-earliestYear+1)
	for y := time.Now().Year(); y >= earliestYear; y-- {
		years = append(years, strconv.Itoa(y))
	}
	return years
}

func cvMonths() []cvMonthOption {
	return []cvMonthOption{
		{"01", "January"}, {"02", "February"}, {"03", "March"},
		{"04", "April"}, {"05", "May"}, {"06", "June"},
		{"07", "July"}, {"08", "August"}, {"09", "September"},
		{"10", "October"}, {"11", "November"}, {"12", "December"},
	}
}

// buildCVEditFormData turns a stored CV into the typed view the edit form
// expects: the JSONB columns are decoded and the semicolon-separated TEXT
// sections are read through verbatim.
func buildCVEditFormData(row db.GetCVByIDRow) (cvForm, error) {
	data := cvForm{ID: row.ID, Title: row.Title}
	unmarshal := func(raw []byte, into any, section string) error {
		if err := json.Unmarshal(raw, into); err != nil {
			return fmt.Errorf("decode %s: %w", section, err)
		}
		return nil
	}

	var err error
	if err = unmarshal(row.Profile, &data.Profile, "profile"); err != nil {
		return data, err
	}

	data.Summary = ""
	if row.Summary.Valid {
		data.Summary = row.Summary.String
	}

	var experiences []cvExperience
	if err = unmarshal(row.Experiences, &experiences, "experiences"); err != nil {
		return data, err
	}
	// We need to rebuild the experience bullet list point to its limit, so if any was unfilled, this create them as empty
	// Otherwise only filled bullet points will be passed down to the data object
	data.Experiences = cvExperienceViews(experiences)

	// Each of these is stored as one semicolon-separated TEXT column and shown
	// verbatim in a single form field, so they need no decoding.
	data.Education = getTextFromPgTypeText(row.Education)
	data.Certifications = getTextFromPgTypeText(row.Certifications)
	data.Academic = getTextFromPgTypeText(row.AcademicContributions)
	data.Skills = getTextFromPgTypeText(row.Skills)

	return data, nil
}

// cvExperienceViews converts stored experiences into the render views the shared
// block partial consumes, splitting each stored "YYYY-MM" back into month/year
// selects and padding bullets so the full set of bullet inputs is shown. The
// json-tagged fields are preserved; only the json:"-" template fields are set.
func cvExperienceViews(stored []cvExperience) []cvExperience {
	views := make([]cvExperience, 0, len(stored))
	for i, exp := range stored {
		startYear, startMonth := splitCVMonthYear(exp.Start)
		endYear, endMonth := splitCVMonthYear(exp.End)

		bullets := make([]string, cvExperienceBulletsLimit)
		copy(bullets, exp.Bullets)

		exp.Index = i
		exp.Months = cvMonths()
		exp.Years = cvYears()
		exp.StartMonth = startMonth
		exp.StartYear = startYear
		exp.EndMonth = endMonth
		exp.EndYear = endYear
		exp.Bullets = bullets

		views = append(views, exp)
	}
	return views
}

// splitCVMonthYear splits a stored "YYYY-MM" date back into its year and month
// parts. A malformed or empty value yields empty parts.
func splitCVMonthYear(date string) (year, month string) {
	parts := strings.SplitN(date, "-", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func buildCreateCVParams(request *http.Request, userID int32) (db.CreateCVParams, error) {
	params := db.CreateCVParams{
		UserID: userID,
	}

	errParse := request.ParseForm()
	if errParse != nil {
		return db.CreateCVParams{}, errors.New("failed to parse form during start of marshalling of cv experiences")
	}
	// The form received by the request can contain many blank fields, so this piece will clean any empty field
	cleanFields := cleanFormFields(request.PostForm)

	var err error
	title := strings.TrimSpace(cleanFields.Get("title"))
	if title == "" {
		slog.Error("Failed to create CV: missing title", "user_id", userID)
		return db.CreateCVParams{}, fmt.Errorf("failed to create cv, missing title")
	}
	params.Title = title

	summary := strings.TrimSpace(cleanFields.Get("summary"))
	params.Summary = pgtype.Text{
		String: summary,
		Valid:  summary != "",
	}

	if params.Profile, err = marshalCVProfile(cleanFields); err != nil {
		return params, fmt.Errorf("parse profile: %w", err)
	}
	if params.Experiences, err = marshalCVExperiences(cleanFields); err != nil {
		return params, fmt.Errorf("parse experiences: %w", err)
	}
	params.Education = pgTextFromForm(cleanFields, "education")
	params.Certifications = pgTextFromForm(cleanFields, "certifications")
	params.AcademicContributions = pgTextFromForm(cleanFields, "academic")
	params.Skills = pgTextFromForm(cleanFields, "skills")

	return params, nil
}

func convertCreateFormToUpdateForm(createForm db.CreateCVParams, idToUpdate, userID int32) db.UpdateCVParams {
	return db.UpdateCVParams{
		ID:                    idToUpdate,
		UserID:                userID,
		Title:                 createForm.Title,
		Summary:               createForm.Summary,
		Profile:               createForm.Profile,
		Experiences:           createForm.Experiences,
		Education:             createForm.Education,
		Certifications:        createForm.Certifications,
		AcademicContributions: createForm.AcademicContributions,
		Skills:                createForm.Skills,
	}
}

// In the form we receive many form fields/values, many could be empty so we extract each one of them and loop through them.
func cleanFormFields(form url.Values) url.Values {
	allKeys := slices.Collect(maps.Keys(form))
	// In case a key is empty, it means the field was not filled in the form, so we drop it
	for _, key := range allKeys {
		if form.Get(key) == "" {
			form.Del(key)
		}
	}
	return form
}

func marshalCVProfile(cleanForm url.Values) ([]byte, error) {
	return json.Marshal(cvProfile{
		Name:     strings.TrimSpace(cleanForm.Get("profile_name")),
		Location: strings.TrimSpace(cleanForm.Get("profile_location")),
	})
}

// The received form contains multiple keys from different fields
func marshalCVExperiences(cleanForm url.Values) ([]byte, error) {
	var experiences []cvExperience
	indexes := extractIndices(cleanForm, "experiences_")

	for _, index := range indexes {
		company := strings.TrimSpace(cleanForm.Get(fmt.Sprintf("experiences_%d_company", index)))
		field := strings.TrimSpace(cleanForm.Get(fmt.Sprintf("experiences_%d_field", index)))
		start := formatCVMonthYear(
			cleanForm.Get(fmt.Sprintf("experiences_%d_start_month", index)),
			cleanForm.Get(fmt.Sprintf("experiences_%d_start_year", index)),
		)
		end := formatCVMonthYear(
			cleanForm.Get(fmt.Sprintf("experiences_%d_end_month", index)),
			cleanForm.Get(fmt.Sprintf("experiences_%d_end_year", index)),
		)
		startParsed, _ := time.Parse("2006-01", start)
		endParsed, _ := time.Parse("2006-01", end)
		if end != "" && endParsed.Before(startParsed) {
			slog.Info("end date is before start date, defaulting end date to start date")
			end = start
		}
		current := cleanForm.Get(fmt.Sprintf("experiences_%d_current", index)) == "true"

		// Doesn't matter the index experience we are, if we try to get up to 5, we'll get all declared, and they can just be empty for edit later
		bullets := make([]string, 0, cvExperienceBulletsLimit)
		for j := 0; j < cvExperienceBulletsLimit; j++ {
			if bullet := strings.TrimSpace(cleanForm.Get(fmt.Sprintf("experiences_%d_bullet_%d", index, j))); bullet != "" {
				bullets = append(bullets, bullet)
			}
		}
		if company != "" || field != "" || start != "" || end != "" || current || len(bullets) > 0 {
			experiences = append(experiences, cvExperience{
				Company: company,
				Field:   field,
				Start:   start,
				End:     end,
				Current: current,
				Bullets: bullets,
			})
		}

	}
	return json.Marshal(experiences)
}

// For each of the fields, this function will be responsible for gathering the indexes provided in the form.
// It's possible the user add fields, delete one in the middle, and the index order is out of sync.
// This extracts only provided ones, so it's possible to loop through relevant indexes.
func extractIndices(cleanForm url.Values, field string) []int32 {
	var indexes []int32

	allKeys := slices.Collect(maps.Keys(cleanForm))
	for _, key := range allKeys {
		if !strings.HasPrefix(key, field) {
			continue
		}

		cutText, found := strings.CutPrefix(key, field)
		if !found {
			continue
		}
		indexStr, _, _ := strings.Cut(cutText, "_")
		indexInt, errAtoi := utils.ConvertFromStrToInt32(indexStr)
		if errAtoi != nil {
			continue
		}
		if slices.Contains(indexes, indexInt) {
			continue
		}
		indexes = append(indexes, indexInt)
	}
	slices.Sort(indexes)
	return indexes
}

func formatCVMonthYear(month, year string) string {
	month = strings.TrimSpace(month)
	year = strings.TrimSpace(year)
	if month == "" || year == "" {
		return ""
	}
	return year + "-" + month
}

// pgTextFromForm reads a semicolon-separated section field and stores it
// verbatim as TEXT, mapping an absent/blank field to NULL.
func pgTextFromForm(cleanForm url.Values, prefix string) pgtype.Text {
	value := cleanForm.Get(prefix)
	return pgtype.Text{
		String: value,
		Valid:  value != "",
	}
}

func getTextFromPgTypeText(textField pgtype.Text) string {
	if textField.Valid {
		return textField.String
	}
	return ""
}
