package xxencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	baseChars = "+-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	lineLen   = 60  // Comprimento da linha codificada
	blockSize = 45  // Tamanho do bloco em bytes
)

// FileInfo contains file metadata from the header
type FileInfo struct {
	Name string
	Mode os.FileMode
}

// NewWriter creates a new XXEncode writer
func NewWriter(w io.Writer, filename string, mode os.FileMode) *Writer {
	ww := &Writer{
		w:        w,
		filename: filename,
		mode:     mode,
		buf:      make([]byte, blockSize),
	}
	fmt.Fprintf(w, "begin %03o %s\n", mode.Perm(), filename)
	return ww
}

// Writer implements XXEncode encoding
type Writer struct {
	w        io.Writer
	filename string
	mode     os.FileMode
	buf      []byte
	n        int
}

func (w *Writer) Write(p []byte) (n int, err error) {
	for len(p) > 0 {
		m := copy(w.buf[w.n:], p)
		w.n += m
		p = p[m:]
		n += m

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

	if _, err := w.w.Write([]byte{baseChars[w.n]}); err != nil {
		return err
	}

	encoded := make([]byte, lineLen)
	for i := 0; i < w.n; i += 3 {
		b0 := w.buf[i]
		b1, b2 := byte(0), byte(0)
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

	if _, err := w.w.Write(encoded[:((w.n+2)/3)*4]); err != nil {
		return err
	}
	_, err := w.w.Write([]byte{'\n'})
	return err
}

func (w *Writer) Flush() error {
	if w.n > 0 {
		if err := w.encodeLine(); err != nil {
			return err
		}
	}
	_, err := w.w.Write([]byte("+\nend\n\n"))
	return err
}

// NewReader creates a new XXEncode reader
func NewReader(r io.Reader, fileInfo *FileInfo) *Reader {
	return &Reader{
		r:        bufio.NewReader(r),
		fileInfo: fileInfo,
	}
}

// Reader implements XXEncode decoding
type Reader struct {
	r          *bufio.Reader
	fileInfo   *FileInfo
	headerRead bool
	eof        bool
	buf        []byte
	pos        int
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

func (r *Reader) Read(p []byte) (n int, err error) {
	if !r.headerRead {
		if err := r.readHeader(); err != nil {
			return 0, err
		}
	}

	if r.eof {
		return 0, io.EOF
	}

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

		length := strings.IndexByte(baseChars, line[0])
		if length == -1 {
			return 0, errors.New("invalid length character")
		}

		data := line[1:]
		r.buf = make([]byte, length)
		bytesDecoded := 0
		requiredChars := ((length + 2) / 3) * 4

		// Verificação rigorosa do comprimento dos dados
		if len(data) < requiredChars {
			// Tenta continuar com o que temos (para lidar com arquivos malformados)
			requiredChars = len(data)
			// Ajusta o comprimento esperado para o que podemos realmente decodificar
			length = (requiredChars / 4) * 3
			if requiredChars%4 >= 1 {
				length++
			}
			if requiredChars%4 >= 2 {
				length++
			}
			r.buf = make([]byte, length)
		}

		for i := 0; i < requiredChars && bytesDecoded < length; i += 4 {
			remaining := requiredChars - i
			if remaining < 4 {
				// Linha incompleta - trata como padding
				break
			}

			c1 := safeIndex(data, i)
			c2 := safeIndex(data, i+1)
			c3 := safeIndex(data, i+2)
			c4 := safeIndex(data, i+3)

			if c1 == -1 || c2 == -1 {
				return 0, fmt.Errorf("invalid encoding at position %d-%d", i, i+1)
			}

			// Primeiro byte
			r.buf[bytesDecoded] = byte((c1 << 2) | (c2 >> 4))
			bytesDecoded++

			if bytesDecoded >= length {
				break
			}

			if c3 == -1 {
				// Padding esperado
				continue
			}

			// Segundo byte
			r.buf[bytesDecoded] = byte((c2 << 4) | (c3 >> 2))
			bytesDecoded++

			if bytesDecoded >= length {
				break
			}

			if c4 == -1 {
				// Padding esperado
				continue
			}

			// Terceiro byte
			r.buf[bytesDecoded] = byte((c3 << 6) | c4)
			bytesDecoded++
		}

		// Ajusta o buffer para o tamanho realmente decodificado
		if bytesDecoded < length {
			r.buf = r.buf[:bytesDecoded]
		}

		r.pos = 0
	}

	n = copy(p, r.buf[r.pos:])
	r.pos += n
	return n, nil
}

// safeIndex retorna o índice do caractere ou -1 se for inválido/fora dos limites
func safeIndex(s string, pos int) int {
	if pos >= len(s) {
		return -1
	}
	return strings.IndexByte(baseChars, s[pos])
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

// MultiWriter para múltiplos arquivos
type MultiWriter struct {
	w io.Writer
}

func NewMultiWriter(w io.Writer) *MultiWriter {
	return &MultiWriter{w: w}
}

func (mw *MultiWriter) WriteFile(filename string, mode os.FileMode, r io.Reader) error {
	ww := NewWriter(mw.w, filename, mode)
	if _, err := io.Copy(ww, r); err != nil {
		return err
	}
	return ww.Flush()
}

func (mw *MultiWriter) Close() error {
	return nil
}

// MultiReader para múltiplos arquivos
type MultiReader struct {
	r       *bufio.Reader
	current *Reader
}

func NewMultiReader(r io.Reader) *MultiReader {
	return &MultiReader{
		r: bufio.NewReader(r),
	}
}

func (mr *MultiReader) Next() (*FileInfo, io.Reader, error) {
	for {
		// Primeiro verifica se temos um arquivo atual em progresso
		if mr.current != nil {
			// Se o arquivo atual terminou, limpe-o antes de procurar o próximo
			if mr.current.eof {
				mr.current = nil
				continue
			}
			return mr.current.fileInfo, mr.current, nil
		}

		// Procura pelo próximo cabeçalho "begin"
		line, err := mr.r.ReadString('\n')
		if err != nil {
			return nil, nil, err
		}

		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "begin ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				mode, err := strconv.ParseUint(fields[1], 8, 32)
				if err != nil {
					return nil, nil, fmt.Errorf("invalid file mode: %v", err)
				}

				mr.current = NewReader(mr.r, &FileInfo{
					Name: fields[2],
					Mode: os.FileMode(mode),
				})
				mr.current.headerRead = true
				return mr.current.fileInfo, mr.current, nil
			}
		}
	}
}
