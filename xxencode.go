// Package xxencode implements XXEncode encoding as specified in the format
package xxencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	baseChars = "+-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	lineLen   = 60  // Comprimento da linha codificada
	blockSize = 45  // Tamanho do bloco em bytes
)

// NewWriter returns a new Writer that writes XXEncoded data to w
func NewWriter(w io.Writer, filename string, mode os.FileMode) *Writer {
	ww := &Writer{
		w:        w,
		filename: filename,
		mode:     mode,
		buf:      make([]byte, blockSize),
	}
	// Escreve o cabeçalho
	fmt.Fprintf(w, "begin %03o %s\n", mode.Perm(), filename)
	return ww
}

type Writer struct {
	w        io.Writer
	filename string
	mode     os.FileMode
	buf      []byte
	n        int
}

func (w *Writer) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		// Preenche o buffer com dados
		m := copy(w.buf[w.n:], p)
		w.n += m
		p = p[m:]
		n += m

		// Se buffer cheio, codifica e escreve
		if w.n == blockSize {
			if err := w.encodeLine(); err != nil {
				return n, err
			}
			w.n = 0
		}
	}
	return n, nil
}

func (w *Writer) encodeLine() error {
	if w.n == 0 {
		return nil
	}

	// Escreve o caractere de comprimento
	_, err := w.w.Write([]byte{baseChars[w.n]})
	if err != nil {
		return err
	}

	// Codifica os dados
	encoded := make([]byte, lineLen)
	for i := 0; i < w.n; i += 3 {
		var b0, b1, b2 byte
		b0 = w.buf[i]
		if i+1 < w.n {
			b1 = w.buf[i+1]
		}
		if i+2 < w.n {
			b2 = w.buf[i+2]
		}

		pos := (i / 3) * 4
		encoded[pos] = baseChars[(b0>>2)&0x3F]
		encoded[pos+1] = baseChars[((b0&0x03)<<4)|(b1>>4)]
		if i+1 < w.n {
			encoded[pos+2] = baseChars[((b1&0x0F)<<2)|(b2>>6)]
		} else {
			encoded[pos+2] = '+'
		}
		if i+2 < w.n {
			encoded[pos+3] = baseChars[b2&0x3F]
		} else {
			encoded[pos+3] = '+'
		}
	}

	// Escreve a linha codificada
	_, err = w.w.Write(encoded[:((w.n+2)/3)*4])
	if err != nil {
		return err
	}
	_, err = w.w.Write([]byte{'\n'})
	return err
}

func (w *Writer) Flush() error {
	// Codifica quaisquer dados restantes
	if w.n > 0 {
		if err := w.encodeLine(); err != nil {
			return err
		}
	}
	// Escreve o terminador
	_, err := w.w.Write([]byte("+\nend\n"))
	return err
}

// NewReader returns a new Reader that reads from r
func NewReader(r io.Reader, fileInfo *FileInfo) *Reader {
	return &Reader{
		r:        bufio.NewReader(r),
		fileInfo: fileInfo,
	}
}

type Reader struct {
	r          *bufio.Reader
	fileInfo   *FileInfo
	headerRead bool
	eof        bool
	buf        []byte
	pos        int
}

type FileInfo struct {
	Name string
	Mode os.FileMode
}

func (r *Reader) readHeader() error {
	if r.headerRead {
		return nil
	}

	for {
		line, err := r.r.ReadString('\n')
		if err != nil {
			return err
		}

		if strings.HasPrefix(line, "begin ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				if r.fileInfo == nil {
					r.fileInfo = &FileInfo{}
				}
				r.fileInfo.Name = fields[2]
				var mode uint32
				fmt.Sscanf(fields[1], "%o", &mode)
				r.fileInfo.Mode = os.FileMode(mode)
				r.headerRead = true
				return nil
			}
		}
	}
}

// Reader implementa a interface io.Reader para decodificar dados XXEncoded
type Reader struct {
	r          *bufio.Reader
	fileInfo   *FileInfo
	headerRead bool
	eof        bool
	buf        []byte
	pos        int
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if !r.headerRead {
		if err := r.readHeader(); err != nil {
			return 0, err
		}
	}

	if r.eof {
		return 0, io.EOF
	}

	// Se buffer vazio, lê próxima linha
	if r.pos >= len(r.buf) {
		line, err := r.r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				r.eof = true
				return 0, io.EOF
			}
			return 0, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "+" {
			r.eof = true
			return 0, io.EOF
		}

		if len(line) == 0 {
			return 0, nil
		}

		// Primeiro caractere é o comprimento
		length := strings.IndexByte(baseChars, line[0])
		if length == -1 {
			return 0, errors.New("invalid length character")
		}

		// Decodifica a linha
		data := line[1:]
		r.buf = make([]byte, 0, length) // Usamos slice e append para controle preciso

		for i := 0; i < len(data); i += 4 {
			if i+3 >= len(data) {
				break
			}

			c1 := strings.IndexByte(baseChars, data[i])
			c2 := strings.IndexByte(baseChars, data[i+1])
			c3 := strings.IndexByte(baseChars, data[i+2])
			c4 := strings.IndexByte(baseChars, data[i+3])

			if c1 == -1 || c2 == -1 {
				return 0, errors.New("invalid encoding character")
			}

			// Decodifica o primeiro byte
			b1 := byte((c1 << 2) | (c2 >> 4))
			r.buf = append(r.buf, b1)

			// Decodifica o segundo byte se disponível
			if len(r.buf) < length && data[i+2] != '+' {
				if c3 == -1 {
					return 0, errors.New("invalid encoding character")
				}
				b2 := byte((c2 << 4) | (c3 >> 2))
				r.buf = append(r.buf, b2)
			}

			// Decodifica o terceiro byte se disponível
			if len(r.buf) < length && data[i+3] != '+' {
				if c4 == -1 {
					return 0, errors.New("invalid encoding character")
				}
				b3 := byte((c3 << 6) | c4)
				r.buf = append(r.buf, b3)
			}
		}

		// Garante que temos exatamente o número de bytes esperados
		if len(r.buf) != length {
			return 0, errors.New("decoded length mismatch")
		}

		r.pos = 0
	}

	// Copia dados do buffer para p
	n = copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

func (r *Reader) File() (string, error) {
	if !r.headerRead {
		if err := r.readHeader(); err != nil {
			return "", err
		}
	}
	return r.fileInfo.Name, nil
}

func (r *Reader) Mode() (os.FileMode, error) {
	if !r.headerRead {
		if err := r.readHeader(); err != nil {
			return 0, err
		}
	}
	return r.fileInfo.Mode, nil
}
