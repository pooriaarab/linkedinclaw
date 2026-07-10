package exportzip

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrIncompleteExport is returned when the zip itself is corrupt or cannot be opened.
var ErrIncompleteExport = errors.New("incomplete or corrupt export zip")

type ConnectionRow struct {
	Urn         string
	FirstName   string
	LastName    string
	Headline    string
	Company     string
	ConnectedAt string
	Source      string
}

type MessageRow struct {
	Urn             string
	ConversationUrn string
	SenderUrn       string
	Body            string
	SentAt          string
	Source          string
}

type ConversationRow struct {
	Urn            string
	Participants   string
	LastActivityAt string
}

type Result struct {
	Connections   []ConnectionRow
	Messages      []MessageRow
	Conversations []ConversationRow
}

var connectionDateLayouts = []string{
	"02 Jan 2006",
	"2 Jan 2006",
	"01/02/2006",
	"1/2/2006",
	"2006-01-02",
	time.RFC3339,
}

var messageDateLayouts = []string{
	"01/02/06, 03:04 PM",
	"01/02/06, 3:04 PM",
	"1/2/06, 3:04 PM",
	"01/02/06, 15:04",
	"01/02/2006, 03:04 PM",
	"01/02/2006, 15:04",
	"2006-01-02 15:04:05",
	time.RFC3339,
}

func parseDate(val string, layouts []string, defaultVal string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultVal
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, val); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return val
}

func isConnectionsHeader(row []string) bool {
	hasFirstName := false
	hasLastName := false
	hasURL := false
	hasConnectedOn := false
	for _, col := range row {
		col = strings.TrimSpace(col)
		if strings.EqualFold(col, "First Name") {
			hasFirstName = true
		} else if strings.EqualFold(col, "Last Name") {
			hasLastName = true
		} else if strings.EqualFold(col, "URL") {
			hasURL = true
		} else if strings.EqualFold(col, "Connected On") {
			hasConnectedOn = true
		}
	}
	return hasFirstName && hasLastName && hasURL && hasConnectedOn
}

func isMessagesHeader(row []string) bool {
	hasConvID := false
	hasSenderURL := false
	hasDate := false
	hasContent := false
	for _, col := range row {
		col = strings.TrimSpace(col)
		if strings.EqualFold(col, "CONVERSATION ID") {
			hasConvID = true
		} else if strings.EqualFold(col, "SENDER PROFILE URL") {
			hasSenderURL = true
		} else if strings.EqualFold(col, "DATE") {
			hasDate = true
		} else if strings.EqualFold(col, "CONTENT") {
			hasContent = true
		}
	}
	return hasConvID && hasSenderURL && hasDate && hasContent
}

func getVal(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

// Parse opens a zip via archive/zip and extracts GDPR data.
func Parse(path string) (Result, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("%w: failed to open zip: %v", ErrIncompleteExport, err)
	}
	defer r.Close()

	var result Result

	type convGroup struct {
		id             string
		participants   map[string]bool
		lastActivityAt string
	}
	conversationsMap := make(map[string]*convGroup)

	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if strings.EqualFold(base, "Connections.csv") {
			rc, err := f.Open()
			if err != nil {
				return Result{}, fmt.Errorf("failed to open Connections.csv in zip: %w", err)
			}
			reader := csv.NewReader(rc)
			reader.FieldsPerRecord = -1
			reader.LazyQuotes = true

			var records [][]string
			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}
				records = append(records, record)
			}
			rc.Close()

			headerIdx := -1
			for i, rec := range records {
				if isConnectionsHeader(rec) {
					headerIdx = i
					break
				}
			}

			if headerIdx != -1 && headerIdx < len(records) {
				headerRow := records[headerIdx]
				firstNameIdx := -1
				lastNameIdx := -1
				urlIdx := -1
				companyIdx := -1
				positionIdx := -1
				connectedOnIdx := -1

				for i, col := range headerRow {
					col = strings.TrimSpace(col)
					switch {
					case strings.EqualFold(col, "First Name"):
						firstNameIdx = i
					case strings.EqualFold(col, "Last Name"):
						lastNameIdx = i
					case strings.EqualFold(col, "URL"):
						urlIdx = i
					case strings.EqualFold(col, "Company"):
						companyIdx = i
					case strings.EqualFold(col, "Position"):
						positionIdx = i
					case strings.EqualFold(col, "Connected On"):
						connectedOnIdx = i
					}
				}

				for i := headerIdx + 1; i < len(records); i++ {
					row := records[i]
					urn := getVal(row, urlIdx)
					if urn == "" {
						continue
					}
					result.Connections = append(result.Connections, ConnectionRow{
						Urn:         urn,
						FirstName:   getVal(row, firstNameIdx),
						LastName:    getVal(row, lastNameIdx),
						Headline:    getVal(row, positionIdx),
						Company:     getVal(row, companyIdx),
						ConnectedAt: parseDate(getVal(row, connectedOnIdx), connectionDateLayouts, ""),
						Source:      "export",
					})
				}
			}
		} else if strings.EqualFold(base, "messages.csv") {
			rc, err := f.Open()
			if err != nil {
				return Result{}, fmt.Errorf("failed to open messages.csv in zip: %w", err)
			}
			reader := csv.NewReader(rc)
			reader.FieldsPerRecord = -1
			reader.LazyQuotes = true

			var records [][]string
			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}
				records = append(records, record)
			}
			rc.Close()

			headerIdx := -1
			for i, rec := range records {
				if isMessagesHeader(rec) {
					headerIdx = i
					break
				}
			}

			if headerIdx != -1 && headerIdx < len(records) {
				headerRow := records[headerIdx]
				convIDIdx := -1
				fromIdx := -1
				senderURLIdx := -1
				toIdx := -1
				recipientURLsIdx := -1
				dateIdx := -1
				contentIdx := -1

				for i, col := range headerRow {
					col = strings.TrimSpace(col)
					switch {
					case strings.EqualFold(col, "CONVERSATION ID"):
						convIDIdx = i
					case strings.EqualFold(col, "FROM"):
						fromIdx = i
					case strings.EqualFold(col, "SENDER PROFILE URL"):
						senderURLIdx = i
					case strings.EqualFold(col, "TO"):
						toIdx = i
					case strings.EqualFold(col, "RECIPIENT PROFILE URLS"):
						recipientURLsIdx = i
					case strings.EqualFold(col, "DATE"):
						dateIdx = i
					case strings.EqualFold(col, "CONTENT"):
						contentIdx = i
					}
				}

				for i := headerIdx + 1; i < len(records); i++ {
					row := records[i]
					convID := getVal(row, convIDIdx)
					if convID == "" {
						continue
					}
					convUrn := "export:conv:" + convID
					sender := getVal(row, senderURLIdx)
					if sender == "" {
						sender = getVal(row, fromIdx)
					}
					body := getVal(row, contentIdx)
					rawDate := getVal(row, dateIdx)
					sentAt := parseDate(rawDate, messageDateLayouts, "")

					hashInput := fmt.Sprintf("%s|%s|%s|%s", convID, sender, sentAt, body)
					sum := sha256.Sum256([]byte(hashInput))
					msgUrn := fmt.Sprintf("export:msg:%x", sum)

					result.Messages = append(result.Messages, MessageRow{
						Urn:             msgUrn,
						ConversationUrn: convUrn,
						SenderUrn:       sender,
						Body:            body,
						SentAt:          sentAt,
						Source:          "export",
					})

					cg, ok := conversationsMap[convUrn]
					if !ok {
						cg = &convGroup{
							id:           convUrn,
							participants: make(map[string]bool),
						}
						conversationsMap[convUrn] = cg
					}

					if sender != "" {
						cg.participants[sender] = true
					}

					recipientsVal := getVal(row, recipientURLsIdx)
					if recipientsVal != "" {
						for _, r := range strings.Split(recipientsVal, ",") {
							r = strings.TrimSpace(r)
							if r != "" {
								cg.participants[r] = true
							}
						}
					} else {
						toVal := getVal(row, toIdx)
						if toVal != "" {
							for _, t := range strings.Split(toVal, ",") {
								t = strings.TrimSpace(t)
								if t != "" {
									cg.participants[t] = true
								}
							}
						}
					}

					if sentAt != "" {
						if cg.lastActivityAt == "" || sentAt > cg.lastActivityAt {
							cg.lastActivityAt = sentAt
						}
					}
				}
			}
		}
	}

	for urn, cg := range conversationsMap {
		var parts []string
		for p := range cg.participants {
			parts = append(parts, p)
		}
		sort.Strings(parts)
		partsStr := strings.Join(parts, ",")

		result.Conversations = append(result.Conversations, ConversationRow{
			Urn:            urn,
			Participants:   partsStr,
			LastActivityAt: cg.lastActivityAt,
		})
	}

	sort.Slice(result.Conversations, func(i, j int) bool {
		return result.Conversations[i].Urn < result.Conversations[j].Urn
	})

	return result, nil
}
