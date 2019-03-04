window.onload = function () {

    let form = document.getElementById('file-form');
    let fileSelect = document.getElementById('file-select');
    let uploadButton = document.getElementById('upload-button');


    form.onsubmit = function(event) {
        event.preventDefault();
        let files = fileSelect.files;
        let formData = new FormData();
        let file = files[0];
        uploadButton.innerHTML = 'Uploading...';
        fetch('ui/api/addManifest', {
            method: 'POST',
            credentials: "same-origin",
            headers: {
                "Content-Type": "text/json"
            },
            body: file
        })
        .then((response) => {
            return response.json()
        })
        .then((response) => {
            if ( response.status != "200") {
                alert("Error uploading: " + response.error)
                uploadButton.innerHTML = 'Try again';
            } else {
                location.reload();
            }
        })
        .catch((error) => {
            console.log(error)
        });
      }

 }

 function RefreshStore () {
    return fetch("/app-store/ui/api/refresh", {
        method: "GET",
        mode: "cors",
        cache: "no-cache",
        credentials: "same-origin",
        headers: {
            "Content-Type": "application/json; charset=utf-8",
        },
    })
    .then(() => {
        console.log("Refressing")
    });
 }

 function UninstallStore(name, giturl) {
    return fetch("/app-store/ui/api/removeStore", {
        method: "POST",
        mode: "cors",
        cache: "no-cache",
        credentials: "same-origin",
        headers: {
            "Content-Type": "application/json; charset=utf-8",
        },
        body: JSON.stringify({"name":name,"giturl":giturl}),
    })
    .then(() => {
        location.reload()
    });
 }

 function AddStore() {
    name = document.getElementById('repoName').value
    giturl = document.getElementById('repoUrl').value
    return fetch("/app-store/ui/api/addStore", {
        method: "POST",
        mode: "cors",
        cache: "no-cache",
        credentials: "same-origin",
        headers: {
            "Content-Type": "application/json; charset=utf-8",
        },
        body: JSON.stringify({"name":name,"giturl":giturl}),
    })
    .then(() => {
        location.reload()
    });
 }