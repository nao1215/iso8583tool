#!/bin/sh
# Writes the inputs of the himorime suite in bench/. Deterministic: the same
# arguments always write the same bytes, so a base and a head revision read
# the same input. Both revisions run the working tree's copy
# (${head_root}/gen.sh).
#
#   sh gen.sh document N FILE   a 0200 message document (the JSON convert
#                               packs) carrying the first N data fields of
#                               the basei-starter spec (2 to 128, without the
#                               bitmap indicator 65 and the hex fields 52 and
#                               64), each at its largest length; N is 1 to
#                               124. Field 55 carries the twenty EMV tags of
#                               the bundled samples.
#   sh gen.sh changed N FILE    the same fields as document N with every
#                               value changed and MTI 0210, for diff
set -eu

case "$1" in
document | changed)
	mkdir -p "$(dirname "$3")"
	# id:type:length for every data field of basei-starter. type is n (fixed,
	# digits), a (fixed, letters and digits), nv (variable digits) and v
	# (variable text), both filled to their maximum, or b (binary, in
	# binary_fields). Values pass validate --strict. The hex fields 52 (PIN
	# data) and 64 (MAC) are left out: a value convert packs for them does
	# not unpack to the same fields again.
	awk -v kind="$1" -v n="$2" '
	function fill(pattern, len,    s) {
		s = ""
		while (length(s) < len) s = s pattern
		return substr(s, 1, len)
	}
	BEGIN {
		spec = "2:pan:19 3:n:6 4:n:12 5:n:12 6:n:12 7:n:10 8:n:8 9:n:8 10:n:8 11:n:6 12:n:6 13:n:4 14:n:4 15:n:4 16:n:4 17:n:4 18:n:4 19:n:3 20:n:3 21:n:3 22:n:3 23:n:3 24:n:3 25:n:2 26:n:2 27:n:1 28:n:9 29:n:9 30:n:9 31:n:9 32:v:11 33:v:11 34:v:28 35:track:37 36:v:104 37:a:12 38:a:6 39:n:2 40:n:3 41:a:8 42:a:15 43:a:40 44:v:99 45:v:76 46:v:999 47:v:999 48:v:999 49:n:3 50:n:3 51:n:3 53:n:16 54:v:120 55:emv:0 56:v:999 57:v:999 58:v:999 59:v:999 60:v:999 61:v:999 62:v:999 63:v:999 66:n:1 67:n:2 68:n:3 69:n:3 70:n:3 71:n:4 72:n:4 73:n:6 74:n:10 75:n:10 76:n:10 77:n:10 78:n:10 79:n:10 80:n:10 81:n:10 82:n:12 83:n:12 84:n:12 85:n:12 86:n:16 87:n:16 88:n:16 89:n:16 90:n:42 91:n:1 92:n:2 93:n:5 94:n:7 95:n:42 96:b:8 97:n:17 98:a:25 99:nv:11 100:nv:11 101:v:17 102:v:28 103:v:28 104:v:100 105:v:999 106:v:999 107:v:999 108:v:999 109:v:999 110:v:999 111:v:999 112:v:999 113:v:999 114:v:999 115:v:999 116:v:999 117:v:999 118:v:999 119:v:999 120:v:999 121:v:999 122:v:999 123:v:999 124:v:999 125:v:999 126:v:999 127:v:999 128:b:8"
		total = split(spec, entries, " ")
		if (n < 1 || n > total) {
			print "gen.sh: a document takes 1 to " total " fields" > "/dev/stderr"
			exit 2
		}
		changed = kind == "changed"
		mti = changed ? "0210" : "0200"
		digits = changed ? "9876543210" : "0123456789"
		letters = changed ? "ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210" : "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		hex = changed ? "FEDCBA9876543210" : "0123456789ABCDEF"
		pan = changed ? "4222222222222222" : "4111111111111111"
		emv = "\"55.5F2A\":\"0392\",\"55.82\":\"3900\",\"55.84\":\"A0000000031010\",\"55.95\":\"8000008000\",\"55.9A\":\"260604\",\"55.9C\":\"00\",\"55.9F02\":\"000000005000\",\"55.9F03\":\"000000000000\",\"55.9F09\":\"008C\",\"55.9F10\":\"06011203A0B0C0\",\"55.9F1A\":\"0392\",\"55.9F1E\":\"5445535431323334\",\"55.9F26\":\"1122334455667788\",\"55.9F27\":\"80\",\"55.9F33\":\"E0F8C8\",\"55.9F34\":\"1F0302\",\"55.9F35\":\"22\",\"55.9F36\":\"0034\",\"55.9F37\":\"11223344\",\"55.9F41\":\"00000001\""
		if (changed) {
			gsub(/0392/, "0840", emv)
			gsub(/000000005000/, "000000009999", emv)
		}
		text = ""; bin = ""
		for (i = 1; i <= n; i++) {
			split(entries[i], e, ":")
			id = e[1]; type = e[2]; len = e[3]
			if (type == "emv") {
				bin = bin (bin == "" ? "" : ",") emv
				continue
			}
			if (type == "b") {
				bin = bin (bin == "" ? "" : ",") "\"" id "\":\"" fill(hex, len * 2) "\""
				continue
			}
			if (type == "pan") value = pan
			else if (type == "track") value = pan "D29122011234567890"
			else if (type == "n" || type == "nv") value = fill(digits, len)
			else if (type == "a") value = fill(letters, len)
			else value = fill((changed ? "field" : "FIELD") id "=abcdefghijklmnopqrstuvwxyz|", len)
			text = text (text == "" ? "" : ",") "\"" id "\":\"" value "\""
		}
		printf "{\"mti\":\"%s\",\"fields\":{%s},\"binary_fields\":{%s}}\n", mti, text, bin
	}' > "$3"
	;;
*)
	echo "gen.sh: unknown kind $1" >&2
	exit 2
	;;
esac
