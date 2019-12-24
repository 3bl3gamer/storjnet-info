export const lang = document.documentElement.lang || 'en'

export function L(defaultText, ...texts) {
	for (let i = 0; i < texts.length; i += 2) {
		if (texts[i] == lang) return texts[i + 1]
	}
	return defaultText
}

function pluralize(val, ...words) {
	if (val < 0) {
		val = -val
	}
	let d0 = val % 10
	let d10 = val % 100
	switch (lang) {
		case 'ru':
			if (d10 == 11 || d10 == 12 || d0 == 0 || (d0 >= 5 && d0 <= 9)) {
				return words[2]
			}
			if (d0 >= 2 && d0 <= 4) {
				return words[1]
			}
			return words[0]
		default:
			if (d10 == 11 || d10 == 12 || d0 == 0 || (d0 >= 2 && d0 <= 9)) {
				return words[1]
			}
			return words[0]
	}
}

L.n = function L_n(val, ...words) {
	return val.toLocaleString(lang) + ' ' + pluralize(val, ...words)
}
