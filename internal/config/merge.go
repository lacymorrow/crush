package config

import (
    "bytes"
    "io"

    "github.com/qjebbs/go-jsons"
)

func Merge(data []io.Reader) (io.Reader, error) {
    // Normalize each reader: if empty (after trimming whitespace), treat as {}
    // so an empty config file doesn't cause a JSON parse error.
    normalized := make([]io.Reader, 0, len(data))
    for _, r := range data {
        b, err := io.ReadAll(r)
        if err != nil {
            return nil, err
        }
        if len(bytes.TrimSpace(b)) == 0 {
            b = []byte("{}")
        }
        normalized = append(normalized, bytes.NewReader(b))
    }

    got, err := jsons.Merge(normalized)
    if err != nil {
        return nil, err
    }
    return bytes.NewReader(got), nil
}
