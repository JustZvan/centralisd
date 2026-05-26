package protocol

import (
    "bufio"
    "errors"
    "strings"
)

func WriteLine(writer *bufio.Writer, line string) error {
    if writer == nil {
        return errors.New("nil writer")
    }
    line = strings.TrimSpace(line)
    if line == "" {
        return errors.New("empty line")
    }
    if _, err := writer.WriteString(line + "\n"); err != nil {
        return err
    }
    return writer.Flush()
}

func ReadLine(reader *bufio.Reader) (string, error) {
    if reader == nil {
        return "", errors.New("nil reader")
    }
    line, err := reader.ReadString('\n')
    if err != nil {
        return "", err
    }
    line = strings.TrimSpace(line)
    if line == "" {
        return "", errors.New("empty line")
    }
    return line, nil
}
