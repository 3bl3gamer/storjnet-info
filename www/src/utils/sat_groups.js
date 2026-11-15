import { useLayoutEffect, useState } from 'preact/hooks'
import { L } from 'src/i18n'
import { getJSONContent } from './elems'

/** Not a real group: stats over all the nodes regardless of satellites */
export const SAT_GROUP_ALL = 'all'
export const SAT_GROUP_OFFICIAL = 'official'

const HASH_PARAM = 'sats'

/**
 * Groups known to the server ('all' goes first), see `core.SatGroups`.
 * @type {string[]}
 */
let satGroups = [SAT_GROUP_ALL]
try {
	satGroups = getJSONContent('sat_groups_data')
} catch (ex) {
	// not the main page
}

export function getSatGroups() {
	return satGroups
}

/** @param {string} group */
export function satGroupLabel(group) {
	if (group === SAT_GROUP_ALL) return L('all', 'ru', 'все')
	if (group === SAT_GROUP_OFFICIAL) return L('official', 'ru', 'официальные')
	return group
}

/** Group selected in the location hash (`#sats=<group>`), 'all' by default. */
export function getHashSatGroup() {
	const group = new URLSearchParams(location.hash.slice(1)).get(HASH_PARAM)
	return group !== null && satGroups.includes(group) ? group : SAT_GROUP_ALL
}

/** @param {string} group */
export function makeUpdatedHashSatGroup(group) {
	const params = new URLSearchParams(location.hash.slice(1))
	params.set(HASH_PARAM, group)
	return '#' + params.toString()
}

/**
 * @param {(group:string) => void} onChange
 */
export function watchHashSatGroup(onChange) {
	let curGroup = getHashSatGroup()
	function listener() {
		const group = getHashSatGroup()
		if (group === curGroup) return
		curGroup = group
		onChange(group)
	}
	function off() {
		removeEventListener('hashchange', listener)
	}
	addEventListener('hashchange', listener)
	return { satGroup: curGroup, off }
}

export function useHashSatGroup() {
	const [group, setGroup] = useState(getHashSatGroup)
	useLayoutEffect(() => {
		const listener = () => setGroup(getHashSatGroup())
		addEventListener('hashchange', listener)
		return () => removeEventListener('hashchange', listener)
	}, [])
	return group
}
