// from https://code.google.com/p/glmatrix/source/browse/glMatrix.js

export const mat4 = {}

mat4.create = function (mat) {
	var dest = new Float32Array(16)

	if (mat) {
		dest[0] = mat[0]
		dest[1] = mat[1]
		dest[2] = mat[2]
		dest[3] = mat[3]
		dest[4] = mat[4]
		dest[5] = mat[5]
		dest[6] = mat[6]
		dest[7] = mat[7]
		dest[8] = mat[8]
		dest[9] = mat[9]
		dest[10] = mat[10]
		dest[11] = mat[11]
		dest[12] = mat[12]
		dest[13] = mat[13]
		dest[14] = mat[14]
		dest[15] = mat[15]
	}

	return dest
}

mat4.ortho = function (left, right, bottom, top, near, far, dest) {
	dest[0] = 2 / (left - right)
	dest[1] = 0
	dest[2] = 0
	dest[3] = 0
	dest[4] = 0
	dest[5] = 2 / (top - bottom)
	dest[6] = 0
	dest[7] = 0
	dest[8] = 0
	dest[9] = 0
	dest[10] = -2 / (far - near)
	dest[11] = 0
	dest[12] = (left + right) / (left - right)
	dest[13] = (top + bottom) / (top - bottom)
	dest[14] = (far + near) / (far - near)
	dest[15] = 1

	return dest
}
