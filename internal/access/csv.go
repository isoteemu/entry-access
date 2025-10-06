package access

import (
	"encoding/csv"
	. "entry-access-control/internal/config"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// CSV based access control

// Definition of fields in CSV access list
type CSVListDefinition struct {
	EmailField  string
	StatusField string

	ActiveStatus string

	Language string // Language code, e.g. "en", "fi"
}

// Known field names in CSV access lists, in different languages
// Note: The actual values should be filled in according to the specific CSV formats used. Funidata might change these nillywilly
var CSVListDefinitions = []CSVListDefinition{
	// English student list definition
	CSVListDefinition{
		EmailField:   "PRIMARY E-MAIL",
		StatusField:  "STUDY RIGHT STATUS",
		ActiveStatus: "Active - Attending",
		Language:     "en",
	},

	// Finnish student list definition
	CSVListDefinition{
		EmailField:   "ENSISIJAINEN SÄHKÖPOSTI",
		StatusField:  "ILMOITTAUTUMISEN TILA",
		ActiveStatus: "Vahvistettu",
		Language:     "fi",
	},
}

type EntryRecord interface {
	GetUserID() string
	CanAccess(EntryID string) bool
}

type StudentEntry struct {
	UserID string
	Email  string
	Status bool
}

func (s *StudentEntry) GetUserID() string {
	return s.UserID
}

func (s *StudentEntry) CanAccess(EntryID string) bool {
	// Check if the student is active
	return s.Status
}

type AccessList interface {
	// stores a nonce with a TTL.
	Find(EntryID string) (EntryRecord, error)
	// Returns true if the nonce existed (valid request), false otherwise.
}

func NewAccessList(typ string) AccessList {
	switch typ {
	case "csv":
		csv := NewCSVAccessList()
		return csv
	default:
		return nil
	}
}

type CSVFile struct {
	FieldDefinitions CSVListDefinition
	HeaderMap        map[string]int
	*csv.Reader
}

type CSVAccessList struct {
	// Lock
	mu sync.RWMutex
	// Map of CSV readers, one per file
	csvReaders map[string]*CSVFile
}

// From entry lists, find if student with EntryID exists
func (s *CSVAccessList) Find(EntryID string) (EntryRecord, error) {
	for _, reader := range s.csvReaders {
		// Search each CSV reader for the EntryID
		// If found, return the corresponding EntryRecord
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("error reading CSV: %w", err)
			}
			if len(record) <= 0 {
				continue
			}

			for i, field := range record {
				if i == reader.HeaderMap[reader.FieldDefinitions.EmailField] {

					// Compare case-insensitively
					if strings.EqualFold(strings.TrimSpace(field), strings.TrimSpace(EntryID)) {
						// Found the entry, check status if applicable
						status := false
						if reader.HeaderMap[reader.FieldDefinitions.StatusField] != -1 {
							status = strings.TrimSpace(record[reader.HeaderMap[reader.FieldDefinitions.StatusField]]) == reader.FieldDefinitions.ActiveStatus
							slog.Debug("Found entry in CSV", slog.String("email", field), slog.Bool("status", status), slog.String("status_field", record[reader.HeaderMap[reader.FieldDefinitions.StatusField]]))
						}
						entry := &StudentEntry{
							UserID: field,
							Email:  field,
							Status: status,
						}
						return entry, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("entry not found")
}

func (s *CSVAccessList) RemoveFile(csvFile string) error {
	for i, reader := range s.csvReaders {
		if reader == nil {
			continue
		}
		// Remove the reader from the map
		if i == csvFile {
			delete(s.csvReaders, i)
			return nil
		}
	}
	return nil
}

// Read CSV file and add entries to access list.
func (c *CSVAccessList) AddFile(csvFile string) error {
	f, err := os.Open(csvFile)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer f.Close()

	// Detect BOM and decode UTF-16 if present. SISU exports UTF-16 with BOM.
	bom := make([]byte, 2)
	n, err := f.Read(bom)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read BOM: %w", err)
	}

	var reader *csv.Reader
	if n == 2 && (bom[0] == 0xFE && bom[1] == 0xFF || bom[0] == 0xFF && bom[1] == 0xFE) {
		// UTF-16 BOM detected
		utf16bom := unicode.BOMOverride(unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
		utf16Reader := transform.NewReader(io.MultiReader(
			// Prepend BOM bytes back to stream
			strings.NewReader(string(bom)),
			f,
		), utf16bom)
		reader = csv.NewReader(utf16Reader)
	} else {
		// No BOM, assume sensible UTF-8
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("failed to seek file: %w", err)
		}
		reader = csv.NewReader(f)
	}

	// Set reader options for tab-delimited, quoted fields
	reader.Comma = '\t'
	reader.LazyQuotes = true
	//reader.FieldsPerRecord = -1
	reader.FieldsPerRecord = 0

	// Read header
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Find index of relevant fields
	var idxStatus, idxEmail int = -1, -1
	var langdef CSVListDefinition
	var csvHeaders = make(map[string]int)

	for _, langdef = range CSVListDefinitions {
		// Reset indexes for each definition
		idxStatus, idxEmail = -1, -1

		// Search for fields in header
		for i, h := range headers {
			csvHeaders[strings.TrimSpace(h)] = i
			switch strings.TrimSpace(h) {
			case langdef.StatusField:
				idxStatus = i
			case langdef.EmailField:
				idxEmail = i
			}
		}
		if idxStatus != -1 && idxEmail != -1 {
			// Found a matching definition
			break
		}
	}
	if idxStatus == -1 || idxEmail == -1 {
		return fmt.Errorf("CSV file missing required fields")
	}

	// Store the reader in the map
	if c.csvReaders == nil {
		c.csvReaders = make(map[string]*CSVFile)
	}
	c.csvReaders[csvFile] = &CSVFile{
		FieldDefinitions: langdef,
		HeaderMap:        csvHeaders,
		Reader:           reader,
	}

	return nil
}

func NewCSVAccessList() *CSVAccessList {
	return &CSVAccessList{}
}

// Scan folder for CSV files and return list of paths.
func getLists(cfg *Config) ([]string, error) {
	var files []string
	root := cfg.AccessListFolder

	// If path is relative, resolve using cwd
	if !filepath.IsAbs(root) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("unable to get current working directory: %w", err)
		}
		root = filepath.Join(cwd, root)
	}

	// Check if folder exists
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("access list folder does not exist: %s", root)
		}
		return nil, fmt.Errorf("error checking access list folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("access list folder is not a directory: %s", root)
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".csv") {
			// TODO Verify that it's a valid access list.
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
