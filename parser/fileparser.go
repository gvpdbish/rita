package parser

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	fpt "github.com/activecm/rita/parser/fileparsetypes"
	pt "github.com/activecm/rita/parser/parsetypes"
	log "github.com/sirupsen/logrus"
)

// readDir recursively reads the directory looking for log and .gz files
func readDir(cpath string, logger *log.Logger) []string {
	var toReturn []string
	files, err := ioutil.ReadDir(cpath)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			"path":  cpath,
		}).Error("Error when reading directory")
	}

	for _, file := range files {
		// Stop RITA from following symlinks
		// In the case that RITA is pointed directly at Bro, it should not
		// parse the "current" symlink which points to the spool.
		// if file.IsDir() && file.Mode() != os.ModeSymlink {
		// 	toReturn = append(toReturn, readDir(path.Join(cpath, file.Name()), logger)...)
		// }
		if !file.IsDir() && strings.HasSuffix(file.Name(), "gz") ||
			strings.HasSuffix(file.Name(), "log") {
			toReturn = append(toReturn, path.Join(cpath, file.Name()))
		}
	}
	return toReturn
}

// readDirRolling recursively reads the directory looking for log and .gz files
// that match the timestamps of the associated chunk
func readDirRolling(currentChunk int, totalChunks int, cpath string, logger *log.Logger) []string {
	var toReturn []string
	files, err := ioutil.ReadDir(cpath)
	if err != nil {
		logger.WithFields(log.Fields{
			"error": err.Error(),
			"path":  cpath,
		}).Error("Error when reading directory")
	}

	frequency := 24 / totalChunks

	startChunk := int64(currentChunk * frequency)
	endChunk := startChunk + int64(frequency)

	for _, file := range files {
		// Stop RITA from following symlinks
		// In the case that RITA is pointed directly at Bro, it should not
		// parse the "current" symlink which points to the spool.
		// if file.IsDir() && file.Mode() != os.ModeSymlink {
		// 	toReturn = append(toReturn, readDir(path.Join(cpath, file.Name()), logger)...)
		// }
		if strings.HasSuffix(file.Name(), "log.gz") {
			fileName := strings.TrimSuffix(file.Name(), ".log.gz")
			split := strings.Split(fileName, ".")
			ts := split[len(split)-1]
			ts = strings.Replace(ts, "-", ":", 1)
			split2 := strings.Split(ts, ":")
			ts1, _ := strconv.ParseInt(split2[0], 10, 32)
			ts2, _ := strconv.ParseInt(split2[3], 10, 32)

			if endChunk == 24 && ts1 == 23 && ts2 == 0 {
				ts2 = 24
			}

			if ts1 == ts2 {
				if (ts1 >= startChunk) && (ts2 <= endChunk) {
					// fmt.Println(file.Name())
					toReturn = append(toReturn, path.Join(cpath, file.Name()))
				}
			} else {

				if (ts1 >= startChunk) && (ts1 < endChunk) &&
					(ts2 > startChunk) && (ts2 <= endChunk) {
					// fmt.Println(file.Name())
					toReturn = append(toReturn, path.Join(cpath, file.Name()))
				}
			}

		} else if strings.HasSuffix(file.Name(), "log") {
			fileName := strings.TrimSuffix(file.Name(), ".log")
			split := strings.Split(fileName, ".")
			ts := split[len(split)-1]
			ts = strings.Replace(ts, "-", ":", 1)
			split2 := strings.Split(ts, ":")
			ts1, _ := strconv.ParseInt(split2[0], 10, 32)
			ts2, _ := strconv.ParseInt(split2[3], 10, 32)

			if endChunk == 24 && ts1 == 23 && ts2 == 0 {
				ts2 = 24
			}

			if ts1 == ts2 {
				if (ts1 >= startChunk) && (ts2 <= endChunk) {
					// fmt.Println(file.Name())
					toReturn = append(toReturn, path.Join(cpath, file.Name()))
				}
			} else {

				if (ts1 >= startChunk) && (ts1 < endChunk) &&
					(ts2 > startChunk) && (ts2 <= endChunk) {
					// fmt.Println(file.Name())
					toReturn = append(toReturn, path.Join(cpath, file.Name()))
				}
			}
		}
	}
	return toReturn
}

// getFileScanner returns a buffered file scanner for a bro log file
func getFileScanner(fileHandle *os.File) (*bufio.Scanner, error) {
	ftype := fileHandle.Name()[len(fileHandle.Name())-3:]
	if ftype != ".gz" && ftype != "log" {
		return nil, errors.New("Filetype not recognized")
	}

	var scanner *bufio.Scanner
	if ftype == ".gz" {
		rdr, err := gzip.NewReader(fileHandle)
		if err != nil {
			return nil, err
		}
		scanner = bufio.NewScanner(rdr)
	} else {
		scanner = bufio.NewScanner(fileHandle)
	}

	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner, nil
}

// scanHeader scans the comment lines out of a bro file and returns a
// BroHeader object containing the information. NOTE: This has the side
// effect of advancing the fileScanner
func scanHeader(fileScanner *bufio.Scanner) (*fpt.BroHeader, error) {
	toReturn := new(fpt.BroHeader)
	for fileScanner.Scan() {
		if fileScanner.Err() != nil {
			break
		}
		if len(fileScanner.Text()) < 1 {
			continue
		}
		//On the comment lines
		if fileScanner.Text()[0] == '#' {
			line := strings.Fields(fileScanner.Text())
			if strings.Contains(line[0], "separator") {
				//TODO: Figure out how to escape read in string
				if line[1] == "\\x09" {
					toReturn.Separator = "\x09"
				}
			} else if strings.Contains(line[0], "set_separator") {
				toReturn.SetSep = line[1]
			} else if strings.Contains(line[0], "empty_field") {
				toReturn.Empty = line[1]
			} else if strings.Contains(line[0], "unset_field") {
				toReturn.Unset = line[1]
			} else if strings.Contains(line[0], "fields") {
				toReturn.Names = line[1:]
			} else if strings.Contains(line[0], "types") {
				toReturn.Types = line[1:]
			} else if strings.Contains(line[0], "path") {
				//remove any tags on the log type
				//i.e. turn http_eth0 into http.
				toReturn.ObjType = stripObjType(line[1])
			}
		} else {
			//We are done parsing the comments
			break
		}
	}

	if len(toReturn.Names) != len(toReturn.Types) {
		return toReturn, errors.New("Name / Type mismatch")
	}
	return toReturn, nil
}

//stripObjType strips a tag of the form "_XXXXX" from the input string.
//Used to remove interface tags from the logs produced by security onion.
//I.e. this function transforms "http_eth0" into "http"
//See: https://github.com/security-onion-solutions/security-onion/issues/205#issue-64090511
func stripObjType(objType string) string {
	idx := strings.LastIndex(objType, "_")
	if idx == -1 {
		return objType
	}
	return objType[0:idx]
}

//mapBroHeaderToParserType checks a parsed BroHeader against
//a BroData struct and returns a mapping from bro field names in the
//bro header to the indexes of the respective fields in the BroData struct
func mapBroHeaderToParserType(header *fpt.BroHeader, broDataFactory func() pt.BroData,
	logger *log.Logger) (fpt.BroHeaderIndexMap, error) {
	// The lookup struct gives us a way to walk the data structure only once
	type lookup struct {
		broType string
		offset  int
	}

	//create a bro data to check the header against
	broData := broDataFactory()

	// map the bro names -> the brotypes
	fieldTypes := make(map[string]lookup)

	//toReturn is a simplified version of the fieldTypes map which
	//links a bro field name to its index in the broData struct
	toReturn := make(map[string]int)

	structType := reflect.TypeOf(broData).Elem()

	// walk the fields of the bro data, making sure the bro data struct has
	// an equal number of named bro fields and bro type
	for i := 0; i < structType.NumField(); i++ {
		structField := structType.Field(i)
		broName := structField.Tag.Get("bro")
		broType := structField.Tag.Get("brotype")

		//If this field is not associated with bro, skip it
		if len(broName) == 0 && len(broType) == 0 {
			continue
		}

		if len(broName) == 0 || len(broType) == 0 {
			return nil, errors.New("incomplete bro variable")
		}
		fieldTypes[broName] = lookup{broType: broType, offset: i}
		toReturn[broName] = i
	}

	// walk the header names array and link each field up with a type in the
	// bro data
	for index, name := range header.Names {
		lu, ok := fieldTypes[name]
		if !ok {
			//NOTE: an unmatched field which exists in the log but not the struct
			//is not a fatal error, so we report it and move on
			logger.WithFields(log.Fields{
				"error":         "unmatched field in log",
				"missing_field": name,
			}).Info("the log contains a field with no candidate in the data structure")
			continue
		}

		if header.Types[index] != lu.broType {
			fmt.Println(header.Types[index], lu.broType)
			return nil, errors.New("Type mismatch found in log")
		}
	}

	return toReturn, nil
}

//parseLine parses a line of a bro log with a given broHeader, fieldMap, into
//the BroData created by the broDataFactory
func parseLine(lineString string, header *fpt.BroHeader,
	fieldMap fpt.BroHeaderIndexMap, broDataFactory func() pt.BroData,
	logger *log.Logger) pt.BroData {
	line := strings.Split(lineString, header.Separator)
	if len(line) < len(header.Names) {
		return nil
	}
	if strings.Contains(line[0], "#") {
		return nil
	}

	dat := broDataFactory()
	data := reflect.ValueOf(dat).Elem()

	for idx, val := range header.Names {
		if line[idx] == header.Empty ||
			line[idx] == header.Unset {
			continue
		}

		//fields not in the struct will not be parsed
		fieldOffset, ok := fieldMap[val]
		if !ok {
			continue
		}

		switch header.Types[idx] {
		case pt.Time:
			secs := strings.Split(line[idx], ".")
			s, err := strconv.ParseInt(secs[0], 10, 64)
			if err != nil {
				logger.WithFields(log.Fields{
					"error": err.Error(),
					"value": line[idx],
				}).Error("Couldn't convert unix ts")
				data.Field(fieldOffset).SetInt(-1)
				break
			}

			n, err := strconv.ParseInt(secs[1], 10, 64)
			if err != nil {
				logger.WithFields(log.Fields{
					"error": err.Error(),
					"value": line[idx],
				}).Error("Couldn't convert unix ts")
				data.Field(fieldOffset).SetInt(-1)
				break
			}

			ttim := time.Unix(s, n)
			tval := ttim.Unix()
			data.Field(fieldOffset).SetInt(tval)
			break
		case pt.String:
			data.Field(fieldOffset).SetString(line[idx])
			break
		case pt.Addr:
			data.Field(fieldOffset).SetString(line[idx])
			break
		case pt.Port:
			pval, err := strconv.ParseInt(line[idx], 10, 32)
			if err != nil {
				logger.WithFields(log.Fields{
					"error": err.Error(),
					"value": line[idx],
				}).Error("Couldn't convert port number")
				data.Field(fieldOffset).SetInt(-1)
				break
			}
			data.Field(fieldOffset).SetInt(pval)
			break
		case pt.Enum:
			data.Field(fieldOffset).SetString(line[idx])
			break
		case pt.Interval:
			flt, err := strconv.ParseFloat(line[idx], 64)
			if err != nil {
				logger.WithFields(log.Fields{
					"error": err.Error(),
					"value": line[idx],
				}).Error("Couldn't convert float")
				data.Field(fieldOffset).SetFloat(-1.0)
				break
			}
			data.Field(fieldOffset).SetFloat(flt)
			break
		case pt.Count:
			cnt, err := strconv.ParseInt(line[idx], 10, 64)
			if err != nil {
				logger.WithFields(log.Fields{
					"error": err.Error(),
					"value": line[idx],
				}).Error("Couldn't convert count")
				data.Field(fieldOffset).SetInt(-1)
				break
			}
			data.Field(fieldOffset).SetInt(cnt)
			break
		case pt.Bool:
			if line[idx] == "T" {
				data.Field(fieldOffset).SetBool(true)
				break
			}
			data.Field(fieldOffset).SetBool(false)
			break
		case pt.StringSet:
			tokens := strings.Split(line[idx], ",")
			tVal := reflect.ValueOf(tokens)
			data.Field(fieldOffset).Set(tVal)
			break
		case pt.EnumSet:
			tokens := strings.Split(line[idx], ",")
			tVal := reflect.ValueOf(tokens)
			data.Field(fieldOffset).Set(tVal)
			break
		case pt.StringVector:
			tokens := strings.Split(line[idx], ",")
			tVal := reflect.ValueOf(tokens)
			data.Field(fieldOffset).Set(tVal)
			break
		case pt.IntervalVector:
			tokens := strings.Split(line[idx], ",")
			floats := make([]float64, len(tokens))
			for i, val := range tokens {
				var err error
				floats[i], err = strconv.ParseFloat(val, 64)
				if err != nil {
					logger.WithFields(log.Fields{
						"error": err.Error(),
						"value": val,
					}).Error("Couldn't convert float")
					break
				}
			}
			fVal := reflect.ValueOf(floats)
			data.Field(fieldOffset).Set(fVal)
			break
		default:
			logger.WithFields(log.Fields{
				"error": "Unhandled type",
				"value": header.Types[idx],
			}).Error("Encountered unhandled type in log")
		}
	}

	return dat
}
