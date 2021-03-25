import { onError } from '../errors'

/**
 * @param {string} value
 */
export function findMeaningfulOctets(value) {
	const m = value.trim().match(/^(\d+\.\d+\.\d+)(?:\.\d+(?:\/24)?)?$/)
	if (m === null) return null
	if (m[1].split('.').some(x => parseInt(x) > 255)) return null
	return m[1]
}

/**
 * @param {string} value
 */
export function isIPv4(value) {
	const m = value.trim().match(/^(\d+)\.(\d+)\.(\d+)\.(\d+)$/)
	return m && m[1] < 256 && m[2] < 256 && m[3] < 256 && m[4] < 256
}

// https://www.iana.org/assignments/dns-parameters/dns-parameters.xhtml#dns-parameters-6
const RESOLVE_STATUS_NAMES_MAP = {
	0: 'No Error',
	1: 'Format Error',
	2: 'Server Failure',
	3: 'Non-Existent Domain',
	4: 'Not Implemented',
	5: 'Query Refused',
	6: 'Name Exists when it should not',
	7: 'RR Set Exists when it should not',
	8: 'RR Set that should exist does not',
	9: 'Server Not Authoritative for zone',
	9: 'Not Authorized',
	10: 'Name not contained in zone',
	11: 'DSO-TYPE Not Implemented',
	// 12-15: Unassigned
	16: 'Bad OPT Version',
	16: 'TSIG Signature Failure',
	17: 'Key not recognized',
	18: 'Signature out of time window',
	19: 'Bad TKEY Mode',
	20: 'Duplicate key name',
	21: 'Algorithm not supported',
	22: 'Bad Truncation',
	23: 'Bad/missing Server Cookie',
	// 24-3840: Unassigned
	// 3841-4095: Reserved for Private Use
	// 4096-65534: Unassigned
	// 65535: Reserved, can be allocated by Standards Action
}

export class ResolveError extends Error {
	constructor(message, response) {
		super(message)
		this.response = response
	}
	get messageLines() {
		return [
			'ResolveError: ' + this.message,
			'Full response: ' + JSON.stringify(this.response, null, '  '),
		]
	}
}

/**
 * @param {string} name
 * @returns {Promise<string[]>}
 */
export function resolve(name) {
	const nameEnc = encodeURIComponent(name)
	const ct = encodeURIComponent('application/dns-json')
	// https://developers.cloudflare.com/1.1.1.1/dns-over-https/json-format
	return fetch(`https://cloudflare-dns.com/dns-query?name=${nameEnc}&type=A&ct=${ct}`)
		.then(r => r.json())
		.then(response => {
			if (response.Status !== 0) {
				const status = RESOLVE_STATUS_NAMES_MAP[response.Status] || 'Unknown Error'
				throw new ResolveError(`Can not resolve ${name}: ${status}`, response)
			}
			let ips = Array.isArray(response.Answer) ? response.Answer.map(x => x.data + '') : []
			if (ips.length === 0 || !ips.every(isIPv4))
				throw new ResolveError(
					`Expected IPv4-addres in Answer[i].data, got ${JSON.stringify(ips)}`,
					response,
				)
			return ips
		})
}

function catchToLog(onLogLines) {
	return err => {
		if (err instanceof ResolveError) {
			onLogLines(...err.messageLines)
		} else {
			onLogLines('Something went wrong: ' + err)
		}
		onError(err)
		return null
	}
}

/**
 * @param {string} name
 * @param {(...lines:string[]) => void} onLogLines
 * @returns {Promise<string|null>}
 */
export function resolveOneOrNull(name, onLogLines) {
	return resolve(name)
		.then(ips => {
			onLogLines(`resolved to [${ips.join(', ')}], using ${ips[0]}`)
			return ips[0]
		})
		.catch(catchToLog)
}

/**
 * @param {string} name
 * @param {(...lines:string[]) => void} onLogLines
 * @returns {Promise<string|null>}
 */
export function resolveSubnetOrNull(name, onLogLines) {
	return resolve(name)
		.then(ips => {
			const subnet = findMeaningfulOctets(ips[0])
			onLogLines(`resolved to [${ips.join(', ')}], using ${subnet}.0`)
			return subnet
		})
		.catch(catchToLog)
}
