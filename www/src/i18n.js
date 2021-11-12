export const lang = document.documentElement.lang || 'en'

export function L(defaultText, ...texts) {
	for (let i = 0; i < texts.length; i += 2) {
		if (texts[i] === lang) return texts[i + 1]
	}
	return defaultText
}

export function pluralize(val, ...words) {
	if (val < 0) {
		val = -val
	}
	let d0 = val % 10
	let d10 = val % 100
	switch (lang) {
		case 'ru':
			if (d10 === 11 || d10 === 12 || d0 === 0 || (d0 >= 5 && d0 <= 9)) {
				return words[2]
			}
			if (d0 >= 2 && d0 <= 4) {
				return words[1]
			}
			return words[0]
		default:
			if (d10 === 11 || d10 === 12 || d0 === 0 || (d0 >= 2 && d0 <= 9)) {
				return words[1]
			}
			return words[0]
	}
}

/**
 * @param {number|null} val
 * @param  {...string} words
 */
L.n = function L_n(val, ...words) {
	return (val === null ? '...' : val.toLocaleString(lang)) + ' ' + pluralize(val || 0, ...words)
}

/**
 * @param {number|null} val
 * @param {string} suffix
 * @param  {...string} words
 */
L.ns = function L_ns(val, suffix, ...words) {
	return (val === null ? '...' : val.toLocaleString(lang)) + suffix + ' ' + pluralize(val || 0, ...words)
}

/**
 * @param {number} duration
 * @returns {string}
 */
export function stringifyDuration(duration) {
	const days = Math.floor(duration / (24 * 3600 * 1000))
	const hours = Math.floor((duration / (3600 * 1000)) % 24)
	const minutes = Math.floor((duration / (60 * 1000)) % 60)
	const seconds = Math.floor((duration / 1000) % 60)
	switch (lang) {
		case 'ru': {
			let res = seconds + ' с'
			if (minutes !== 0) res = minutes + ' мин ' + res
			if (hours !== 0) res = hours + ' ч ' + res
			if (days !== 0) res = days + ' д ' + res
			return res
		}
		default: {
			let res = seconds + ' s'
			if (minutes !== 0) res = minutes + ' min ' + res
			if (hours !== 0) res = hours + ' h ' + res
			if (days !== 0) res = days + ' d ' + res
			return res
		}
	}
}
/**
 * @param {Date|number} date
 * @returns {string}
 */
export function ago(date) {
	return stringifyDuration(Date.now() - +date)
}
