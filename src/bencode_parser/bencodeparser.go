package bencodeparser

import (
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"strconv"
)

func get_last(stack []string) string {
	if len(stack) == 0 {
		return ""
	}

	return stack[len(stack)-1]
}

type ParsedData struct {
	Data           any
	Info_idx_start int
	Info_idx_end   int
	Info_hash      [20]byte
}

type ParsedList []any
type ParsedDict map[string]any
type ParsedStr string
type ParsedInt int

var (
	parsed_data ParsedData = ParsedData{}
)

func ParseFile(filename string) (ParsedData, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return parsed_data, fmt.Errorf("E: Opening file. %s", err.Error())
	}

	val, _ := parse(0, &data)
	parsed_data.Data = val
	parsed_data.Info_hash = sha1.Sum(data[parsed_data.Info_idx_start:parsed_data.Info_idx_end])

	return parsed_data, nil
}

func main() {
	data, err := os.ReadFile("example_gta.torrent")
	if err != nil {
		log.Panicln("E: Opening torrent file", err.Error())
	}

	// var dict_pairs map[string]any
	val, _ := parse(0, &data)

	switch v := val.(type) {
	case ParsedStr:
		fmt.Println("string:", v)
	case ParsedInt:
		fmt.Println("int:", v)
	case ParsedList:
		fmt.Println("list:", v)
	case ParsedDict:
		PrettyPrint(v, 0)
		// fmt.Println("dict:", v)
		// for k, vl := range v {
		// 	// fmt.Printf("%s %#v\n", k, vl)
		// }
	}

	// fmt.Printf("dict_pairs: %v\n", dict_pairs)
	// for k, v := range dict_pairs {
	// 	fmt.Printf("%s %s\n", k, v)
	// }
}

func parse(idx int, data *[]byte) (any, int) {
	switch (*data)[idx] {
	case 'd':
		return parse_dict(idx, data)

	case 'l':
		return parse_list(idx, data)

	case 'i':
		return parse_int(idx, data)

	default:
		return parse_str(idx, data)
	}
}

func parse_dict(idx int, data *[]byte) (ParsedDict, int) {
	// skip 'd'
	idx += 1

	var res ParsedDict = make(map[string]any)

	for (*data)[idx] != 'e' {
		// parse the key first
		// is always a string
		pair_key, new_idx := parse_str(idx, data)
		if pair_key == "info" {
			parsed_data.Info_idx_start = new_idx
		}
		idx = new_idx

		// parse the value
		// can be anything
		var pair_val any
		pair_val, new_idx = parse(idx, data)
		if pair_key == "info" {
			parsed_data.Info_idx_end = new_idx
		}
		idx = new_idx

		res[pair_key] = pair_val
		// fmt.Println("\nCURR RES")
		// for k, v := range res {
		// 	fmt.Printf("%s -> %v\n", k, v)
		// }
	}

	// skip 'e'
	idx += 1

	return res, idx
}

func parse_str(idx int, data *[]byte) (string, int) {
	// str_val, idx := parse_str(idx, data)
	str_len := ""

	for (*data)[idx] != ':' {
		c := string((*data)[idx])
		_, err := strconv.Atoi(c)
		if err != nil {
			// if its not a num, just pack it up bro
			fmt.Println("idx before crashing at", idx, string((*data)[idx]))
			log.Panicln("Error parsing string len num, cannot proceed", err.Error())
		}

		str_len += c
		idx += 1
	}

	// move to first char
	idx += 1
	start_idx := idx
	// at last char of string
	str_len_int, err := strconv.Atoi(str_len)
	if err != nil {
		log.Panicln("Error parsing string len, cannot proceed", err.Error())
	}

	end_idx := idx + str_len_int - 1
	idx = end_idx + 1

	return string((*data)[start_idx : end_idx+1]), idx
}

// idx at 'i'
func parse_int(idx int, data *[]byte) (int, int) {
	i := idx + 1
	str_val := ""
	c := string((*data)[i])

	for {
		// currently at i + 1
		c = string((*data)[i])
		if c == "e" {
			break
		}

		str_val += c
		i += 1
	}

	int_val, err := strconv.Atoi(str_val)
	if err != nil {
		log.Panicln("E: Could not parse to integer", err.Error())
	}

	// skip 'e'
	i += 1

	return int_val, i
}

func parse_list(idx int, data *[]byte) (ParsedList, int) {
	var res ParsedList

	// skip 'l'
	idx += 1

	for (*data)[idx] != 'e' {
		list_val, new_idx := parse(idx, data)
		idx = new_idx
		res = append(res, list_val)
	}

	// skip 'e'
	idx += 1

	return res, idx
}

func PrettyPrint(val any, indent int) {
	prefix := ""
	for i := 0; i < indent; i++ {
		prefix += "  "
	}

	switch v := val.(type) {

	case string:
		fmt.Println(prefix + v)

	case int:
		fmt.Println(prefix + strconv.Itoa(v))

	case []any:
		for _, item := range v {
			fmt.Println(prefix + "-")
			PrettyPrint(item, indent+1)
		}

	case ParsedDict:
		for key, value := range v {
			switch value.(type) {

			case ParsedDict, []any:
				fmt.Println(prefix + key + ":")
				PrettyPrint(value, indent+1)

			default:
				fmt.Printf("%s%s: ", prefix, key)
				PrettyPrint(value, 0)
			}
		}

	default:
		fmt.Println(prefix + "unknown type")
	}
}
