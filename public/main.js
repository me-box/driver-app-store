window.onload = function () {

    let form = document.getElementById('file-form');
    let fileSelect = document.getElementById('file-select');
    let uploadButton = document.getElementById('upload-button');


    form.onsubmit = function(event) {
        event.preventDefault();
        let files = fileSelect.files;
        let formData = new FormData();
        let file = files[0];
        if (!file.type.match(/json/i)) {
            alert("Incorrect file type")
            return
        }
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