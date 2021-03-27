import { Provider, connect as connect_ } from 'unistore/src/integrations/preact'
import { html } from './htm'

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
