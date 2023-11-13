// If the page was opened with an anchor (e.g. #foo),
// and the destination is a <details> element, open it.
function openDetailsAnchor() {
	let hash = window.location.hash
	if (!hash) {
		return
	}
	let el = document.getElementById(hash.slice(1)) // remove leading '#'
	if (!el) {
		return
	}
	if (el.tagName == "DETAILS") {
		el.open = true
	}
}

window.addEventListener('hashchange', () => openDetailsAnchor())

window.addEventListener('load', () => {
	document.querySelectorAll("h2, h3, h4, h5, h6").forEach((el) => {
		if (!el.id) {
			return
		}
		el.innerHTML += ' <a class="permalink" href="#'+el.id+'">&para;</a>'
	})

	document.querySelectorAll("details.example > summary").forEach((el) => {
		let id = el.parentElement.id;
		if (!id) {
			return
		}
		el.innerHTML += ' <a class="permalink" href="#'+id+'">&para;</a>'
	})

	openDetailsAnchor()
})
