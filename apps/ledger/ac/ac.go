package ac

import (
	"encoding/json"
	"github.com/keep94/consume2"
	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/aggregators"
	"github.com/keep94/finances/fin/consumers"
	"github.com/keep94/finances/fin/findb"
	"github.com/keep94/toolbox/http_util"
	"net/http"
)

const (
	kMaxAutoComplete = 1000
)

type Handler struct {
	Store findb.EntriesRunner
	Field func(e fin.Entry) string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	aca := &aggregators.AutoCompleteAggregator{Field: h.Field}
	acc := consumers.FromEntryAggregator(aca)
	acc = consume2.Slice(acc, 0, kMaxAutoComplete)
	err := h.Store.Entries(nil, nil, acc)
	if err != nil {
		http_util.ReportError(w, "Error reading database.", err)
		return
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(aca.Items)
}
