// Package annotate translates well-known ISO 8583 / EMV coded values into
// short human-readable meanings. It is intentionally pure data: no I/O, no
// dependency on the moov message types, so it can be reused by any renderer.
package annotate

import (
	"encoding/hex"
	"strings"
)

// The four MTI position vocabularies (version, class, function, origin). Kept at
// package scope so MTI() does not re-allocate them on every call.
var (
	mtiVersion = map[byte]string{
		'0': "ISO8583:1987", '1': "ISO8583:1993", '2': "ISO8583:2003",
		'8': "National", '9': "Private",
	}
	mtiClass = map[byte]string{
		'1': "Authorization", '2': "Financial", '3': "File action",
		'4': "Reversal", '5': "Reconciliation", '6': "Administrative",
		'7': "Fee collection", '8': "Network management", '9': "Reserved",
	}
	mtiFunction = map[byte]string{
		'0': "Request", '1': "Response", '2': "Advice",
		'3': "Advice response", '4': "Notification", '5': "Notification ack",
		'6': "Instruction", '7': "Instruction ack",
	}
	mtiOrigin = map[byte]string{
		'0': "Acquirer", '1': "Acquirer repeat", '2': "Issuer",
		'3': "Issuer repeat", '4': "Other", '5': "Other repeat",
	}
)

// MTI returns a human description of a 4-digit Message Type Indicator,
// or "" when the value does not look like an MTI.
func MTI(mti string) string {
	mti = strings.TrimSpace(mti)
	if len(mti) != 4 || !isDigits(mti) {
		return ""
	}

	parts := make([]string, 0, 3)
	if v, ok := mtiClass[mti[1]]; ok {
		parts = append(parts, v)
	}
	if v, ok := mtiFunction[mti[2]]; ok {
		parts = append(parts, v)
	}
	if v, ok := mtiOrigin[mti[3]]; ok {
		parts = append(parts, "from "+v)
	}
	head := strings.Join(parts, " ")
	if v, ok := mtiVersion[mti[0]]; ok {
		if head == "" {
			return v
		}
		return head + " (" + v + ")"
	}
	return head
}

// FieldMeaning returns a short description for a coded field value, keyed by
// its dot-path (for example "39", "49", "55.5F2A"). The boolean reports
// whether a meaning was found.
func FieldMeaning(path, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}

	// A constructed TLV tag carries the same meaning at any nesting depth: an
	// 8A (ARC) under 55.70 means what an 8A directly under 55 means. Collapse the
	// nested path to its container+leaf form ("55.70.8A" -> "55.8A") so the table
	// below, and the helpers it calls, key off the leaf tag, not the template.
	path = normalizeFieldPath(path)

	switch path {
	case "3": // processing code (6 digits; moov may strip leading zeros)
		return transactionType(leftPad(value, 6))
	case "55.9C": // EMV transaction type
		return transactionType(value)
	case "39", "55.8A": // response code (F39 ASCII, ARC packed ASCII hex)
		return responseCode(path, value)
	case "49", "55.5F2A": // currency code (numeric ISO 4217)
		if name, ok := currencyCodes[normalizeNumeric(value)]; ok {
			return name, true
		}
	case "55.9F1A", "19": // country code (numeric ISO 3166)
		if name, ok := countryCodes[normalizeNumeric(value)]; ok {
			return name, true
		}
	case "22": // POS entry mode
		return posEntryMode(value)
	case "25": // POS condition code
		if name, ok := posConditionCodes[twoDigits(value)]; ok {
			return name, true
		}
	case "70": // network management information code
		if name, ok := networkManagementCodes[leftPad(value, 3)]; ok {
			return name, true
		}
	case "55.9F27": // Cryptogram Information Data
		return cryptogramInfo(value)
	case "7": // Transmission date & time MMDDhhmmss
		return formatStamp(value, "MM-DD hh:mm:ss", []int{2, 2, 2, 2, 2})
	case "12": // Local transaction time hhmmss
		return formatStamp(value, "hh:mm:ss", []int{2, 2, 2})
	case "13": // Local transaction date MMDD
		return formatStamp(value, "MM-DD", []int{2, 2})
	case "14": // Expiration date YYMM
		return formatStamp(value, "20YY-MM", []int{2, 2})
	case "55.9A": // EMV transaction date YYMMDD
		return formatStamp(value, "20YY-MM-DD", []int{2, 2, 2})
	}
	return "", false
}

// normalizeFieldPath collapses a nested TLV path to the container+leaf form the
// meaning table is keyed by, so "55.70.8A" is treated like "55.8A". A top-level
// path ("39") or a single-level path ("55.8A") is returned unchanged.
func normalizeFieldPath(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) <= 2 {
		return path
	}
	return parts[0] + "." + parts[len(parts)-1]
}

// formatStamp renders a fixed-width numeric date/time by slicing the value into
// the given segment widths and substituting them, left to right, into the
// placeholder tokens (YY, MM, DD, hh, mm, ss) found in the layout.
func formatStamp(value, layout string, widths []int) (string, bool) {
	value = strings.TrimSpace(value)
	total := 0
	for _, w := range widths {
		total += w
	}
	value = leftPad(value, total)
	if len(value) != total {
		return "", false
	}

	segments := make([]string, len(widths))
	pos := 0
	for i, w := range widths {
		segments[i] = value[pos : pos+w]
		pos += w
	}

	var out strings.Builder
	seg := 0
	for i := 0; i < len(layout); {
		if i+2 <= len(layout) && seg < len(segments) && isStampToken(layout[i:i+2]) {
			out.WriteString(segments[seg])
			seg++
			i += 2
			continue
		}
		out.WriteByte(layout[i])
		i++
	}
	return out.String(), true
}

func isStampToken(tok string) bool {
	switch tok {
	case "YY", "MM", "DD", "hh", "mm", "ss":
		return true
	default:
		return false
	}
}

// FormatAmount renders a minor-unit amount using the currency's decimal
// exponent, for example ("000000005000", "392") -> "JPY 5000" and
// ("000000005000", "840") -> "USD 50.00".
func FormatAmount(amount, currencyNumeric string) (string, bool) {
	alpha, ok := CurrencyAlpha(currencyNumeric)
	if !ok {
		return "", false
	}
	digits := strings.TrimLeft(strings.TrimSpace(amount), "0")
	if digits == "" {
		digits = "0"
	}
	exp := currencyExponent(normalizeNumeric(currencyNumeric))
	if exp == 0 {
		return alpha + " " + digits, true
	}
	for len(digits) <= exp {
		digits = "0" + digits
	}
	whole := digits[:len(digits)-exp]
	frac := digits[len(digits)-exp:]
	return alpha + " " + whole + "." + frac, true
}

// CurrencyAlpha returns the ISO 4217 alpha code (e.g. "JPY") for a numeric code.
func CurrencyAlpha(currencyNumeric string) (string, bool) {
	name, ok := currencyCodes[normalizeNumeric(currencyNumeric)]
	if !ok {
		return "", false
	}
	return strings.Fields(name)[0], true
}

func currencyExponent(numeric string) int {
	switch numeric {
	case "392", "410": // JPY, KRW have no minor unit
		return 0
	default:
		return 2
	}
}

func transactionType(value string) (string, bool) {
	if name, ok := transactionTypes[twoDigits(value)]; ok {
		return name, true
	}
	return "", false
}

func responseCode(path, value string) (string, bool) {
	code := value
	if path == "55.8A" { // ARC is the hex of two ASCII characters, e.g. "3030" -> "00"
		if decoded, err := hex.DecodeString(value); err == nil && len(decoded) > 0 {
			code = string(decoded)
		}
	}
	if name, ok := responseCodes[strings.TrimSpace(code)]; ok {
		return name, true
	}
	return "", false
}

func posEntryMode(value string) (string, bool) {
	if name, ok := posEntryModes[twoDigits(value)]; ok {
		if len(value) >= 3 {
			switch value[2] {
			case '1':
				return name + "; PIN entry capable", true
			case '2':
				return name + "; no PIN entry", true
			}
		}
		return name, true
	}
	return "", false
}

func cryptogramInfo(value string) (string, bool) {
	b, err := hex.DecodeString(value)
	if err != nil || len(b) == 0 {
		return "", false
	}
	switch b[0] & 0xC0 {
	case 0x00:
		return "AAC (declined offline)", true
	case 0x40:
		return "TC (approved offline)", true
	case 0x80:
		return "ARQC (online authorization requested)", true
	default:
		return "RFU", true
	}
}

// normalizeNumeric trims leading zeros and left-pads to 3 digits so that
// "392", "0392", and "036" all resolve consistently.
func normalizeNumeric(value string) string {
	value = strings.TrimSpace(value)
	trimmed := strings.TrimLeft(value, "0")
	if trimmed == "" {
		trimmed = "0"
	}
	for len(trimmed) < 3 {
		trimmed = "0" + trimmed
	}
	return trimmed
}

func leftPad(value string, width int) string {
	value = strings.TrimSpace(value)
	for len(value) < width {
		value = "0" + value
	}
	return value
}

func twoDigits(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		return value[:2]
	}
	return value
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

var transactionTypes = map[string]string{
	"00": "Purchase / goods & services",
	"01": "Cash withdrawal",
	"09": "Purchase with cashback",
	"10": "Account funding",
	"17": "Cash disbursement",
	"20": "Refund / return",
	"28": "Payment",
	"30": "Balance inquiry",
	"40": "Cardholder account transfer",
}

var responseCodes = map[string]string{
	"00": "Approved",
	"01": "Refer to card issuer",
	"02": "Refer to issuer, special condition",
	"03": "Invalid merchant",
	"04": "Pick up card",
	"05": "Do not honour",
	"06": "Error",
	"07": "Pick up card, special condition",
	"08": "Honour with identification",
	"10": "Approved (partial)",
	"11": "Approved (VIP)",
	"12": "Invalid transaction",
	"13": "Invalid amount",
	"14": "Invalid card number",
	"15": "No such issuer",
	"19": "Re-enter transaction",
	"21": "No action taken",
	"25": "Unable to locate record",
	"30": "Format error",
	"41": "Lost card",
	"43": "Stolen card",
	"51": "Insufficient funds",
	"54": "Expired card",
	"55": "Incorrect PIN",
	"57": "Transaction not permitted to cardholder",
	"58": "Transaction not permitted to terminal",
	"59": "Suspected fraud",
	"61": "Exceeds withdrawal amount limit",
	"62": "Restricted card",
	"63": "Security violation",
	"65": "Exceeds withdrawal frequency limit",
	"75": "PIN tries exceeded",
	"76": "Invalid/nonexistent account",
	"91": "Issuer or switch inoperative",
	"92": "Financial institution not found",
	"94": "Duplicate transmission",
	"96": "System malfunction",
}

var posEntryModes = map[string]string{
	"00": "Unknown entry mode",
	"01": "Manual / key entered",
	"02": "Magnetic stripe",
	"03": "Bar code",
	"05": "Integrated circuit card (ICC)",
	"07": "Contactless ICC",
	"79": "Contactless magnetic stripe",
	"80": "Fallback to magnetic stripe",
	"90": "Magnetic stripe (full track)",
	"91": "Contactless magnetic stripe",
	"95": "ICC, CVV may be unreliable",
}

var posConditionCodes = map[string]string{
	"00": "Normal presentment",
	"01": "Customer not present",
	"02": "Unattended terminal",
	"03": "Merchant suspicious",
	"05": "Customer present, card not present",
	"06": "Pre-authorized request",
	"08": "Mail / telephone order",
	"51": "Account verification",
	"59": "E-commerce transaction",
}

var networkManagementCodes = map[string]string{
	"001": "Sign on",
	"002": "Sign off",
	"161": "Session key change",
	"162": "New key",
	"201": "Cutover",
	"301": "Echo test",
}

var currencyCodes = map[string]string{
	"036": "AUD (Australian dollar)",
	"124": "CAD (Canadian dollar)",
	"156": "CNY (Chinese yuan)",
	"344": "HKD (Hong Kong dollar)",
	"356": "INR (Indian rupee)",
	"392": "JPY (Japanese yen)",
	"410": "KRW (South Korean won)",
	"484": "MXN (Mexican peso)",
	"554": "NZD (New Zealand dollar)",
	"643": "RUB (Russian ruble)",
	"702": "SGD (Singapore dollar)",
	"710": "ZAR (South African rand)",
	"752": "SEK (Swedish krona)",
	"756": "CHF (Swiss franc)",
	"764": "THB (Thai baht)",
	"784": "AED (UAE dirham)",
	"826": "GBP (Pound sterling)",
	"840": "USD (US dollar)",
	"978": "EUR (Euro)",
	"986": "BRL (Brazilian real)",
}

var countryCodes = map[string]string{
	"036": "Australia",
	"124": "Canada",
	"156": "China",
	"250": "France",
	"276": "Germany",
	"344": "Hong Kong",
	"356": "India",
	"392": "Japan",
	"410": "South Korea",
	"484": "Mexico",
	"554": "New Zealand",
	"643": "Russia",
	"702": "Singapore",
	"826": "United Kingdom",
	"840": "United States",
}
