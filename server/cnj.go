package main

import (
	"regexp"
	"strconv"
)

var nonDigit = regexp.MustCompile(`\D`)

func onlyDigits(s string) string { return nonDigit.ReplaceAllString(s, "") }

// formatCNJ: 20 digitos -> NNNNNNN-DD.AAAA.J.TT.OOOO
func formatCNJ(cnj string) string {
	d := onlyDigits(cnj)
	if len(d) != 20 {
		return cnj
	}
	return d[0:7] + "-" + d[7:9] + "." + d[9:13] + "." + d[13:14] + "." + d[14:16] + "." + d[16:20]
}

// isValidCNJ: confere o digito verificador (ISO 7064 MOD 97-10).
func isValidCNJ(cnj string) bool {
	d := onlyDigits(cnj)
	if len(d) != 20 {
		return false
	}
	seq, dd, ano, seg, tt, origem := d[0:7], d[7:9], d[9:13], d[13:14], d[14:16], d[16:20]
	base := seq + ano + seg + tt + origem
	n, err := strconv.ParseInt(base, 10, 64)
	if err != nil {
		return false
	}
	resto := (n * 100) % 97
	dv := 98 - resto
	expected := strconv.FormatInt(dv, 10)
	if len(expected) == 1 {
		expected = "0" + expected
	}
	return expected == dd
}

// Justica Estadual: codigo TT (digitos 14-16) -> alias DataJud.
var aliasByCode = map[string]string{
	"01": "api_publica_tjac", "02": "api_publica_tjal", "03": "api_publica_tjap",
	"04": "api_publica_tjam", "05": "api_publica_tjba", "06": "api_publica_tjce",
	"07": "api_publica_tjdft", "08": "api_publica_tjes", "09": "api_publica_tjgo",
	"10": "api_publica_tjma", "11": "api_publica_tjmt", "12": "api_publica_tjms",
	"13": "api_publica_tjmg", "14": "api_publica_tjpa", "15": "api_publica_tjpb",
	"16": "api_publica_tjpr", "17": "api_publica_tjpe", "18": "api_publica_tjpi",
	"19": "api_publica_tjrj", "20": "api_publica_tjrn", "21": "api_publica_tjrs",
	"22": "api_publica_tjro", "23": "api_publica_tjrr", "24": "api_publica_tjsc",
	"25": "api_publica_tjse", "26": "api_publica_tjsp", "27": "api_publica_tjto",
}

var ufToCode = map[string]string{
	"AC": "01", "AL": "02", "AP": "03", "AM": "04", "BA": "05", "CE": "06",
	"DF": "07", "ES": "08", "GO": "09", "MA": "10", "MT": "11", "MS": "12",
	"MG": "13", "PA": "14", "PB": "15", "PR": "16", "PE": "17", "PI": "18",
	"RJ": "19", "RN": "20", "RS": "21", "RO": "22", "RR": "23", "SC": "24",
	"SE": "25", "SP": "26", "TO": "27",
}

// aliasFromCNJ resolve o indice DataJud pelo segmento (J) + tribunal (TT).
func aliasFromCNJ(cnj string) string {
	d := onlyDigits(cnj)
	if len(d) != 20 {
		return ""
	}
	j := d[13:14]
	tt := d[14:16]
	switch j {
	case "8": // estadual
		return aliasByCode[tt]
	case "4": // federal -> TRF
		n, _ := strconv.Atoi(tt)
		if n >= 1 && n <= 6 {
			return "api_publica_trf" + strconv.Itoa(n)
		}
	case "5": // trabalho -> TRT (TT=00 -> TST)
		n, _ := strconv.Atoi(tt)
		if n == 0 {
			return "api_publica_tst"
		}
		if n >= 1 && n <= 24 {
			return "api_publica_trt" + strconv.Itoa(n)
		}
	case "3":
		return "api_publica_stj"
	case "7":
		return "api_publica_stm"
	}
	return ""
}
