/**
 * @template T
 * @param {T|null} val
 * @returns T
 */
export function mustBeNotNull(val) {
	if (val === null) throw new Error('value must not be null')
	return val
}

/**
 * @template T
 * @param {Promise<T>|unknown} value
 * @returns {value is Promise<T>}
 */
export function isPromise(value) {
	return typeof value === 'object' && value !== null && 'then' in value
}
