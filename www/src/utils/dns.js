import { onError } from '../errors'

export function findMeaningfulOctets(value) {
	const m = value.trim().match(/^(\d+\.\d+\.\d+)(?:\.\d+(?:\/24)?)?$/)
	if (m === null) return null
	if (m[1].split('.').some(x => parseInt(x) > 255)) return null
	return m[1]
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

/**
 * @param {string} name
 * @param {(...lines:string[]) => void} onLogLines
 * @returns {Promise<string>}
 */
export function resolveSubnet(name, onLogLines) {
	const nameEnc = encodeURIComponent(name)
	const ct = encodeURIComponent('application/dns-json')
	// https://developers.cloudflare.com/1.1.1.1/dns-over-https/json-format
	return fetch(`https://cloudflare-dns.com/dns-query?name=${nameEnc}&type=A&ct=${ct}`)
		.then(r => r.json())
		.then(response => {
			if (response.Status !== 0) {
				const status = RESOLVE_STATUS_NAMES_MAP[response.Status] || 'Unknown Error'
				onLogLines(
					`Error: Can not resolve ${name}: ${status}`,
					'Full response: ' + JSON.stringify(response, null, '  '),
				)
				return null
			}
			const ip = (response.Answer && response.Answer[0] && response.Answer[0].data) + ''
			const subnet = findMeaningfulOctets(ip)
			if (subnet === null) {
				onLogLines(
					`Error: Expected IPv4-addres in Answer[0].data, got ${ip}`,
					'Full response: ' + JSON.stringify(response, null, '  '),
				)
				return null
			}
			onLogLines(`resolved to [${response.Answer.map(x => x.data).join(', ')}], using ${subnet}.0`)
			return subnet
		})
		.catch(err => {
			onLogLines('Something went wrong: ' + err)
			throw err
		})
}

/**
 * @param {string} name
 * @param {(...lines:string[]) => void} onLogLines
 * @returns {Promise<string|null>}
 */
export function resolveSubnetOrNull(name, onLogLines) {
	return resolveSubnet(name, onLogLines).catch(err => {
		onError(err)
		return null
	})
}
