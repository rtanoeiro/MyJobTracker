package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"job-applications/internal/db"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type MockCVStore struct {
	ShouldFailCreateCV      bool
	ShouldFailDeleteCV      bool
	ShouldFailGetAllUserCVs bool
	ShouldFailGetCVByID     bool
	ShouldFailIsCVOwner     bool
	ShouldFailUpdateCV      bool

	IsOwner bool

	CVs []db.GetAllUserCVsRow
	CV  db.GetCVByIDRow

	CreatedCV db.CreateCVParams
}

func (m *MockCVStore) CreateCV(_ context.Context, arg db.CreateCVParams) (int32, error) {
	m.CreatedCV = arg
	if m.ShouldFailCreateCV {
		return 0, fmt.Errorf("mock error: failed to create cv")
	}
	return 10, nil
}

func (m *MockCVStore) DeleteCV(_ context.Context, _ db.DeleteCVParams) error {
	if m.ShouldFailDeleteCV {
		return fmt.Errorf("mock error: failed to delete cv")
	}
	return nil
}

func (m *MockCVStore) GetAllUserCVs(_ context.Context, _ int32) ([]db.GetAllUserCVsRow, error) {
	if m.ShouldFailGetAllUserCVs {
		return nil, fmt.Errorf("mock error: failed to list cvs")
	}
	return m.CVs, nil
}

func (m *MockCVStore) GetCVByID(_ context.Context, _ db.GetCVByIDParams) (db.GetCVByIDRow, error) {
	if m.ShouldFailGetCVByID {
		return db.GetCVByIDRow{}, fmt.Errorf("mock error: cv not found")
	}
	return m.CV, nil
}

func (m *MockCVStore) IsCVOwner(_ context.Context, _ db.IsCVOwnerParams) (bool, error) {
	if m.ShouldFailIsCVOwner {
		return false, fmt.Errorf("mock error: failed to check owner")
	}
	return m.IsOwner, nil
}

func (m *MockCVStore) UpdateCV(_ context.Context, _ db.UpdateCVParams) error {
	if m.ShouldFailUpdateCV {
		return fmt.Errorf("mock error: failed to update cv")
	}
	return nil
}

func newCVHandler(store CVStore, renderShouldFail bool) *CVHandler {
	return NewCVHandler(store, newMockRenderer(renderShouldFail))
}

func cvStoreWith(row db.GetCVByIDRow, isOwner bool) *MockCVStore {
	return &MockCVStore{CV: row, IsOwner: isOwner}
}

// rendererFailingOnTemplate lets tests fail a specific template render while
// letting earlier renders succeed, e.g. the "+ Add" button that follows a
// successfully rendered snippet block.
type rendererFailingOnTemplate struct {
	failOn string
}

func (r *rendererFailingOnTemplate) Render(_ io.Writer, name string, _ any) error {
	if name == r.failOn {
		return fmt.Errorf("mock render error on %s", name)
	}
	return nil
}

func TestCVPage_NoUserID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs", nil)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVPage_RenderError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, true)
	req := httptest.NewRequest(http.MethodGet, "/cvs", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVPage_Success(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCVCreateForm_RenderError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, true)
	req := httptest.NewRequest(http.MethodGet, "/cvs/new", nil)
	w := httptest.NewRecorder()

	handler.CreateCVForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCreateForm_Success(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs/new", nil)
	w := httptest.NewRecorder()

	handler.CreateCVForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCVCreate_NoUserID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := formRequest(http.MethodPost, "/cvs", url.Values{"title": {"CV"}})
	w := httptest.NewRecorder()

	handler.CreateCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCreate_MissingTitle(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := formRequest(http.MethodPost, "/cvs", url.Values{})
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CreateCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCreate_DBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailCreateCV: true}, false)
	req := formRequest(http.MethodPost, "/cvs", url.Values{"title": {"CV"}})
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CreateCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCreate_Success(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := formRequest(http.MethodPost, "/cvs", url.Values{"title": {"CV"}})
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CreateCV(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if redirect := w.Header().Get("HX-Redirect"); redirect != "/account/cvs" {
		t.Errorf("HX-Redirect = %q, want /account/cvs", redirect)
	}
}

func TestCVList_NoUserID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs/list", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVList_DBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailGetAllUserCVs: true}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVList_RenderError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, true)
	req := httptest.NewRequest(http.MethodGet, "/cvs/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVList_Success(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := httptest.NewRequest(http.MethodGet, "/cvs/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCVRenderEditForm_InvalidID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)

	for _, id := range []string{"abc", "0"} {
		req := requestWithURLParam(http.MethodGet, "/cvs/"+id+"/edit", "id", id)
		w := httptest.NewRecorder()

		handler.RenderEditForm(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("id %q: status = %d, want %d", id, w.Code, http.StatusBadRequest)
		}
	}
}

func TestCVRenderEditForm_NoUserID(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), false), false)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVRenderEditForm_DBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailGetCVByID: true, IsOwner: true}, false)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVRenderEditForm_DecodeError(t *testing.T) {
	badRow := cvRow()
	badRow.Profile = []byte(`{"name":`)
	handler := newCVHandler(cvStoreWith(badRow, true), false)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVRenderEditForm_RenderError(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), true)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVRenderEditForm_NotOwner(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), false), false)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCVRenderEditForm_Success(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := requestWithURLParam(http.MethodGet, "/cvs/7/edit", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.RenderEditForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCVUpdate_InvalidID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/abc", url.Values{"title": {"CV"}}, "id", "abc")
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVUpdate_NoUserID(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/7", url.Values{"title": {"CV"}}, "id", "7")
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVUpdate_NotOwner(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), false), false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/7", url.Values{"title": {"CV"}}, "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCVUpdate_MissingTitle(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/7", url.Values{}, "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVUpdate_DBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailUpdateCV: true, IsOwner: true}, false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/7", url.Values{"title": {"CV"}}, "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVUpdate_Success(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := formRequestWithURLParam(http.MethodPut, "/cvs/7", url.Values{"title": {"CV"}}, "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateCV(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if redirect := w.Header().Get("HX-Redirect"); redirect != "/account/cvs" {
		t.Errorf("HX-Redirect = %q, want /account/cvs", redirect)
	}
}

func TestCVDelete_InvalidID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/abc", "id", "abc")
	w := httptest.NewRecorder()

	handler.DeleteCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVDelete_NoUserID(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/7", "id", "7")
	w := httptest.NewRecorder()

	handler.DeleteCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVDelete_NotOwner(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), false), false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/7", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.DeleteCV(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCVDelete_DBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailDeleteCV: true, IsOwner: true}, false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/7", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.DeleteCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVDelete_Success(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/7", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.DeleteCV(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if redirect := w.Header().Get("HX-Redirect"); redirect != "/account/cvs" {
		t.Errorf("HX-Redirect = %q, want /account/cvs", redirect)
	}
}

func TestCVCopy_InvalidID(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := requestWithURLParam(http.MethodPost, "/cvs/abc/copy", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVCopy_NoUserID(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVCopy_NotOwner(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), false), false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestCVCopy_GetDBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailGetCVByID: true, IsOwner: true}, false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCopy_CreateDBError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{ShouldFailCreateCV: true, IsOwner: true, CV: cvRow()}, false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVCopy_Success(t *testing.T) {
	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if redirect := w.Header().Get("HX-Redirect"); redirect != "/account/cvs" {
		t.Errorf("HX-Redirect = %q, want /account/cvs", redirect)
	}
}

func TestCVCopy_PrefixesTitle(t *testing.T) {
	store := cvStoreWith(cvRow(), true)
	handler := newCVHandler(store, false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if got := store.CreatedCV.Title; got != "Copy of Generalist" {
		t.Errorf("copied title = %q, want %q", got, "Copy of Generalist")
	}
}

func TestCVCopy_CopiesAllSectionsVerbatim(t *testing.T) {
	source := cvRow()
	source.UserID = 1
	source.Summary = pgtype.Text{String: "A short summary", Valid: true}

	store := cvStoreWith(source, true)
	handler := newCVHandler(store, false)
	req := requestWithURLParam(http.MethodPost, "/cvs/7/copy", "id", "7")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.CopyCV(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	got := store.CreatedCV
	if got.UserID != 1 {
		t.Errorf("UserID = %d, want 1", got.UserID)
	}
	if got.Summary != source.Summary {
		t.Errorf("Summary = %+v, want %+v", got.Summary, source.Summary)
	}

	jsonSections := []struct {
		name string
		got  []byte
		want []byte
	}{
		{"Profile", got.Profile, source.Profile},
		{"Experiences", got.Experiences, source.Experiences},
	}
	for _, section := range jsonSections {
		if !bytes.Equal(section.got, section.want) {
			t.Errorf("%s = %s, want %s", section.name, section.got, section.want)
		}
	}

	textSections := []struct {
		name string
		got  pgtype.Text
		want pgtype.Text
	}{
		{"Education", got.Education, source.Education},
		{"Certifications", got.Certifications, source.Certifications},
		{"AcademicContributions", got.AcademicContributions, source.AcademicContributions},
		{"Skills", got.Skills, source.Skills},
	}
	for _, section := range textSections {
		if section.got != section.want {
			t.Errorf("%s = %+v, want %+v", section.name, section.got, section.want)
		}
	}
}

func TestCVIsOwner(t *testing.T) {
	ctx := context.Background()

	handler := newCVHandler(cvStoreWith(cvRow(), true), false)
	if owner := handler.isCVOwner(ctx, 7, 1); !owner {
		t.Error("owner: expected true for the CV's owner")
	}

	handler = newCVHandler(cvStoreWith(cvRow(), false), false)
	if owner := handler.isCVOwner(ctx, 7, 1); owner {
		t.Error("non-owner: expected false when CV belongs to another user")
	}

	handler = newCVHandler(&MockCVStore{ShouldFailIsCVOwner: true}, false)
	if owner := handler.isCVOwner(ctx, 7, 1); owner {
		t.Error("store error: expected false")
	}
}

func TestCVAddSnippetBlock_InvalidSection(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := formRequestWithURLParam(http.MethodPost, "/cvs/snippets/hobbies/add", url.Values{"next_index": {"0"}}, "section", "hobbies")
	w := httptest.NewRecorder()

	handler.AddSnippetBlock(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCVAddSnippetBlock_InvalidIndex(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)

	for _, nextIndex := range []string{"", "abc", "-1"} {
		req := formRequestWithURLParam(http.MethodPost, "/cvs/snippets/experiences/add", url.Values{"next_index": {nextIndex}}, "section", "experiences")
		w := httptest.NewRecorder()

		handler.AddSnippetBlock(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("next_index %q: status = %d, want %d", nextIndex, w.Code, http.StatusBadRequest)
		}
	}
}

func TestCVAddSnippetBlock_AllSections(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)

	for _, section := range ALLOWED_SECTIONS {
		req := formRequestWithURLParam(http.MethodPost, "/cvs/snippets/"+section+"/add", url.Values{"next_index": {"2"}}, "section", section)
		w := httptest.NewRecorder()

		handler.AddSnippetBlock(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("section %q: status = %d, want %d", section, w.Code, http.StatusOK)
		}
	}
}

func TestCVAddSnippetBlock_RenderError(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, true)
	req := formRequestWithURLParam(http.MethodPost, "/cvs/snippets/experiences/add", url.Values{"next_index": {"0"}}, "section", "experiences")
	w := httptest.NewRecorder()

	handler.AddSnippetBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVAddSnippetBlock_AddButtonRenderError(t *testing.T) {
	renderer := &rendererFailingOnTemplate{failOn: "cv-add-experiences-button"}
	handler := NewCVHandler(&MockCVStore{}, renderer)
	req := formRequestWithURLParam(http.MethodPost, "/cvs/snippets/experiences/add", url.Values{"next_index": {"0"}}, "section", "experiences")
	w := httptest.NewRecorder()

	handler.AddSnippetBlock(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestCVRemoveSnippetBlock_InvalidIndex(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)

	for _, index := range []string{"abc", "-1"} {
		req := requestWithURLParam(http.MethodDelete, "/cvs/snippets/experiences/"+index, "index", index)
		w := httptest.NewRecorder()

		handler.RemoveSnippetBlock(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("index %q: status = %d, want %d", index, w.Code, http.StatusBadRequest)
		}
	}
}

func TestCVRemoveSnippetBlock_Success(t *testing.T) {
	handler := newCVHandler(&MockCVStore{}, false)
	req := requestWithURLParam(http.MethodDelete, "/cvs/snippets/experiences/2", "index", "2")
	w := httptest.NewRecorder()

	handler.RemoveSnippetBlock(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestValidateSections(t *testing.T) {
	for _, section := range ALLOWED_SECTIONS {
		if err := validateSections(section); err != nil {
			t.Errorf("section %q should be allowed, got error %v", section, err)
		}
	}

	for _, section := range []string{"", "experience", "Projects", "hobbies"} {
		if err := validateSections(section); err == nil {
			t.Errorf("section %q should be rejected", section)
		}
	}
}
