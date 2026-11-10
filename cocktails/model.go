package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

type item struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type cocktail struct {
	item
	Instructions []struct {
		Amount     int
		Ingredient item
	}
}

type ingredient struct {
	Name      string
	Cocktails []item
}

type stock struct {
	item
	InFridge bool `json:"inFridge"`
}

type ratingInput struct {
	Stars   int    `json:"stars"`
	Comment string `json:"comment"`
}

type rating struct {
	ID           string
	CocktailID   uint
	CocktailName string
	CreatedAt    time.Time
	ratingInput
}

// api kapselt den HTTP-Vertrag. Normale Anfragen haben ein Zeitlimit;
// der Live-Stream endet dagegen erst beim Schließen des Feed-Fensters.
type api struct {
	base   string
	client http.Client
}

func (a *api) request(ctx context.Context, method, path string, input any) (*http.Response, error) {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+"/api"+path, body)
	if err != nil {
		return nil, err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		var result struct{ Error string }
		if json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&result) == nil && result.Error != "" {
			return nil, fmt.Errorf("%s", result.Error)
		}
		return nil, fmt.Errorf("Server antwortet mit %s", resp.Status)
	}
	return resp, nil
}

func (a *api) json(ctx context.Context, method, path string, input, output any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.request(ctx, method, path, input)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if output != nil {
		return json.NewDecoder(resp.Body).Decode(output)
	}
	return nil
}

// stream sammelt alle data-Zeilen bis zur Leerzeile, wie es SSE vorsieht.
func (a *api) stream(ctx context.Context, event func(string, string)) error {
	resp, err := a.request(ctx, http.MethodGet, "/ratings/stream", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	media, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if media != "text/event-stream" {
		return fmt.Errorf("Server liefert keinen Ereignisstream")
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(splitSSELine)
	var kind string
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(data) > 0 {
				event(kind, strings.Join(data, "\n"))
			}
			kind, data = "", nil
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			kind = value
		case "data":
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

// SSE akzeptiert LF, CRLF und einzelne CR als Zeilenende.
func splitSSELine(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		end := i + 1
		if data[i] == '\r' {
			if end == len(data) && !atEOF {
				return 0, nil, nil
			}
			if end < len(data) && data[end] == '\n' {
				end++
			}
		}
		return end, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
