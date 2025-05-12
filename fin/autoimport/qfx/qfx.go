// Package qfx provides processing of QFX files
package qfx

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/keep94/finances/fin"
	"github.com/keep94/finances/fin/autoimport"
	"github.com/keep94/finances/fin/autoimport/qfx/qfxdb"
	"github.com/keep94/toolbox/date_util"
	"github.com/keep94/toolbox/db"
)

const (
	kDtPosted     = "<DTPOSTED>"
	kTrnAmt       = "<TRNAMT>"
	kName         = "<NAME>"
	kMemo         = "<MEMO>"
	kCheckNum     = "<CHECKNUM>"
	kStmtTrnClose = "</STMTTRN>"
	kFitId        = "<FITID>"
)

var (
	kQFXHeaderPattern = regexp.MustCompile(`^\s*[A-Z]+:[A-Z0-9]+\s*$`)
	kXMLTagPattern    = regexp.MustCompile(`</?[A-Z]+>`)
)

// QFXLoader implements the autoimport.Loader interface for QFX files.
type QFXLoader struct {
	// Store stores which fitIds, unique identifier in QFX files,
	// have already been processed.
	Store qfxdb.Store
}

func (q QFXLoader) Load(
	accountId int64,
	bankAccountId string,
	r io.Reader,
	startDate time.Time) (autoimport.Batch, error) {
	scanner := bufio.NewScanner(r)
	var line string
	var err error

	// skip over QFX headers for now
	for scanner.Scan() {
		line = scanner.Text()
		if !kQFXHeaderPattern.MatchString(line) {
			break
		}
	}

	// Load the XML body into this buffer
	var qfxContents bytes.Buffer
	for scanner.Scan() {
		line = scanner.Text()
		line = strings.TrimSpace(line)
		qfxContents.Write([]byte(line))
	}

	// Return any errors from reading the file.
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	// We break the XML body into a stream of tags and contents.
	xmlContents := qfxContents.Bytes()
	allTagIndexes := kXMLTagPattern.FindAllIndex(xmlContents, -1)
	tagCount := len(allTagIndexes)

	qe := &QfxEntry{}
	var result []*QfxEntry
	var readName, readMemo string

	for i := 0; i < tagCount; i++ {
		tag := string(xmlContents[allTagIndexes[i][0]:allTagIndexes[i][1]])
		var contents string
		if i+1 < tagCount {
			contents = string(
				xmlContents[allTagIndexes[i][1]:allTagIndexes[i+1][0]])
		} else {
			contents = string(xmlContents[allTagIndexes[i][1]:])
		}
		if tag == kDtPosted {
			qe.Date, err = parseQFXDate(contents)
			if err != nil {
				return nil, err
			}
		} else if tag == kName {
			readName = strings.Replace(contents, "&amp;", "&", -1)
		} else if tag == kMemo {
			readMemo = strings.Replace(contents, "&amp;", "&", -1)
		} else if tag == kCheckNum {
			qe.CheckNo = contents
		} else if tag == kTrnAmt {
			var amt int64
			amt, err = fin.ParseUSD(contents)
			if err != nil {
				return nil, err
			}
			qe.CatPayment = fin.NewCatPayment(fin.Expense, -amt, true, accountId)
		} else if tag == kFitId {
			qe.FitId = contents
		} else if tag == kStmtTrnClose {
			// No meaningful contents with this closing tag. This closing tag
			// means that we are done with an entry.
			if !qe.Date.Before(startDate) {
				// Prefer name field to memo field
				if strings.TrimSpace(readName) != "" {
					qe.Name = readName
				} else {
					qe.Name = readMemo
				}
				err = qe.Check()
				if err != nil {
					return nil, err
				}
				result = append(result, qe)
			}
			qe = &QfxEntry{}
			readName = ""
			readMemo = ""
		}
	}
	return &QfxBatch{Store: q.Store, AccountId: accountId, QfxEntries: result}, nil
}

// QfxBatch implements the autoimport.Batch interface. Although it was
// written for QFX files, it can be reused for any import file type as
// long as each transaction has a unique ID like the fitId in QFX files.
// QfxBatch instances must be treated as immutable.
type QfxBatch struct {
	// Stores the fitIds of imported transactions.
	Store qfxdb.Store

	// The account ID to where entries in this batch will be imported
	AccountId int64

	// The entries to be imported along with their fitIds
	QfxEntries []*QfxEntry
}

func (q *QfxBatch) Entries() []fin.Entry {
	result := make([]fin.Entry, len(q.QfxEntries))
	for i := range q.QfxEntries {
		result[i] = q.QfxEntries[i].Entry
	}
	return result
}

func (q *QfxBatch) Len() int {
	return len(q.QfxEntries)
}

func (q *QfxBatch) SkipProcessed(t db.Transaction) (autoimport.Batch, error) {
	existingFitIds, err := q.Store.Find(t, q.AccountId, q.toFitIdSet())
	if err != nil {
		return nil, err
	}
	if existingFitIds == nil {
		return q, nil
	}
	result := make([]*QfxEntry, len(q.QfxEntries))
	idx := 0
	for _, qe := range q.QfxEntries {
		if !existingFitIds[qe.FitId] {
			result[idx] = qe
			idx++
		}
	}
	return &QfxBatch{Store: q.Store, AccountId: q.AccountId, QfxEntries: result[:idx]}, nil
}

func (q *QfxBatch) MarkProcessed(t db.Transaction) error {
	return q.Store.Add(t, q.AccountId, q.toFitIdSet())
}

func (q *QfxBatch) toFitIdSet() qfxdb.FitIdSet {
	fitIdSet := make(qfxdb.FitIdSet, len(q.QfxEntries))
	for _, qe := range q.QfxEntries {

		// Only report FITIDs that aren't 0. This way 0 never gets recorded
		// as a used FITID, and 0 never get read as an existing FITID.
		if strings.TrimSpace(qe.FitId) != "0" {
			fitIdSet[qe.FitId] = true
		}
	}
	return fitIdSet
}

// QfxEntry represents an entry to be imported along with its fitId.
type QfxEntry struct {
	fin.Entry
	FitId string
}

// Check ensures this instance contains required fields.
func (q *QfxEntry) Check() error {
	if strings.TrimSpace(q.Name) == "" {
		return errors.New("Imported entry missing name field.")
	}
	if q.FitId == "" {
		return errors.New("Imported entry missing fit id.")
	}
	return nil
}

func parseQFXDate(s string) (time.Time, error) {
	if len(s) < 8 {
		return time.Time{}, errors.New("Invalid date field in qfx file.")
	}
	return time.Parse(date_util.YMDFormat, s[:8])
}
