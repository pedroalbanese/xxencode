# XXE
[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](https://github.com/pedroalbanese/xxencode/blob/master/LICENSE.md) 
[![GoDoc](https://godoc.org/github.com/pedroalbanese/xxencode?status.png)](http://godoc.org/github.com/pedroalbanese/xxencode)
[![GitHub downloads](https://img.shields.io/github/downloads/pedroalbanese/xxencode/total.svg?logo=github&logoColor=white)](https://github.com/pedroalbanese/xxencode/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pedroalbanese/xxencode)](https://goreportcard.com/report/github.com/pedroalbanese/xxencode)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/pedroalbanese/xxencode)](https://golang.org)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/pedroalbanese/xxencode)](https://github.com/pedroalbanese/xxencode/releases)  
### XXEncode is a tool that converts to and from xxencoding

## Format
<pre>XXE format:
  permission mode _______       ______ file name to be given to decoded file
                         |     |
  begin line ____ begin 644 filename
                  5ABCD5EFGH5IJKL5MNOP5QRST5UVWX5YZab5cdef5ghij5klmn5opqr5st
  encoded data __ 5uvwx5yz01+2345-67895ABCD5EFGH5IJKL5MNOP5QRST5UVWX5YZabcd
                  +
  end line ______ end</pre>
  
## Usage
<pre>
Usage of xxe:
  -d    Decode instead of Encode
  -f string
        Target file</pre>
 
## License
This project is licensed under the ISC License. 
 
**Copyright (c) 2025 Pedro Albanese <pedroalbanese@hotmail.com>**
