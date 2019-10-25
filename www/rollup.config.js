import nodeResolve from 'rollup-plugin-node-resolve'

function mustImport(name) {
	return import(name).catch(err => {
		throw err
	})
}

export default function(commandOptions) {
	const isProd = process.env.NODE_ENV === 'production'

	let devPlugins = []
	if (!isProd)
		devPlugins.push(
			mustImport('rollup-plugin-serve').then(({ default: serve }) =>
				serve({
					contentBase: 'dist',
					host: commandOptions.configHost || 'localhost',
					port: commandOptions.configPort || '12345',
				}),
			),
			mustImport('rollup-plugin-livereload').then(({ default: livereload }) =>
				livereload({ verbose: true, watch: 'dist/bundle.js' }),
			),
		)

	return Promise.all(devPlugins).then(devPlugins => ({
		input: 'src/index.js',
		output: {
			format: 'esm',
			dir: 'dist',
			entryFileNames: isProd ? 'bundle.[hash].js' : 'bundle.js',
			sourcemap: true,
		},
		plugins: [
			...devPlugins,
			css({ output: `dist/bundle${isProd ? '.[hash]' : ''}.css` }),
			// commonjs({}), //rollup-plugin-commonjs
			nodeResolve({}),
		],
		watch: { clearScreen: false },
	}))
}

import { createFilter } from 'rollup-pluginutils'
import { promises as fs } from 'fs'
import path from 'path'
import Concat from 'concat-with-sourcemaps'
import crypto from 'crypto'

function css(options = {}) {
	const filter = createFilter(options.include || ['**/*.css'], options.exclude)
	const styles = {}
	let output = options.output
	let changes = 0

	return {
		name: 'css',
		transform(code, id) {
			if (!filter(id)) return

			// Keep track of every stylesheet
			// Check if it changed since last render
			if (styles[id] !== code && (styles[id] || code)) {
				styles[id] = code
				changes++
			}

			return ''
		},
		generateBundle(opts) {
			// No stylesheet needed
			if (!changes) return
			changes = 0

			// Combine all stylesheets
			let concat = new Concat(true, output, '\n\n')
			for (const id in styles) {
				concat.add(id, styles[id])
			}
			let hash = crypto
				.createHash('md5')
				.update(concat.content)
				.digest('hex')
				.substr(0, 8)
			let contentFPath = output.replace('[hash]', hash)
			let sourceMapFPath = contentFPath + '.map'
			let dirname = path.dirname(contentFPath)
			let content =
				concat.content + `\n/*# sourceMappingURL=${path.basename(sourceMapFPath)} */`
			let sourceMap = concat.sourceMap

			return fs
				.mkdir(dirname, { recursive: true })
				.then(() =>
					Promise.all([
						fs.writeFile(contentFPath, content),
						fs.writeFile(sourceMapFPath, sourceMap),
					]),
				)
		},
	}
}
