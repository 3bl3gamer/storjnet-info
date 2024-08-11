import { Provider, connect as connect_ } from 'unistore/src/integrations/preact'
import { html } from './htm'
import { useCallback, useRef, useState } from 'preact/hooks'

// fixig types. could import it just from unistore/preact, but will have to add rollup commonjs plugin
const connect = /** @type {import('unistore/preact').connect} */ (/**@type {*}*/ (connect_))

export function connectAndWrap(Comp, store, mapStateToProps, actions) {
	let ConnectedComp = connect(mapStateToProps, actions)(Comp)
	return () => html`
		<${Provider} store=${store}>
			<${ConnectedComp} group="my" />
		<//>
	`
}

/**
 * @template T
 * @param {string} key
 * @param {(storedVal:unknown) => T} initialState
 * @returns {[T, import('preact/hooks').StateUpdater<T>]}
 */
export function useStorageState(key, initialState) {
	const [val, setValInner] = useState(() => {
		let storedVal = undefined
		try {
			const item = localStorage.getItem(key)
			storedVal = item === null ? null : JSON.parse(item)
		} catch (ex) {
			// eslint-disable-next-line no-console
			console.error(ex)
		}
		return initialState(storedVal)
	})

	const keyRef = useRef(key)
	keyRef.current = key

	const setVal = useCallback((/**@type {T | ((prevState: T) => T)}*/ v) => {
		setValInner(oldVal => {
			// @ts-expect-error
			const newVal = typeof v === 'function' ? v(oldVal) : v
			try {
				localStorage.setItem(keyRef.current, JSON.stringify(newVal))
			} catch (ex) {
				// eslint-disable-next-line no-console
				console.error(ex)
			}
			return newVal
		})
	}, [])

	return [val, setVal]
}
