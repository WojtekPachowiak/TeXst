// render.js – Vanilla JS polling for async rendering

const form = document.getElementById('renderForm');
const engineSelect = document.getElementById('engine');
const formatSelect = document.getElementById('format');
const submitBtn = document.getElementById('submitBtn');
const spinner = document.getElementById('spinner');
const statusDiv = document.getElementById('status');
const previewDiv = document.getElementById('preview');

const JobStatusQueued = "queued"
const JobStatusUploading = "uploading"
const JobStatusProcessing = "processing"
const JobStatusFailed = "failed"
const JobStatusCompleted = "completed"



// code editor
var codeEditor = CodeMirror.fromTextArea(document.getElementById('source'), {
    lineNumbers: true,
    mode: 'stex',
    theme: 'ayu-dark',
    width: "100%",
    heighg : "100%",
    // extraKeys: {"F11": toggleFullscreenEditing, "Esc": toggleFullscreenEditing},

});
codeEditor.setSize("100%", "100%");

// Helper: show status message (can be error or info)
function setStatus(message, isError = false) {
    statusDiv.innerHTML = message;
    statusDiv.style.color = isError ? 'red' : 'unset';
}

// Helper: show/hide spinner
function setLoading(loading) {
    if (loading) {
        spinner.style.display = 'inline-block';
        submitBtn.disabled = true;
        formatSelect.disabled = true;
        engineSelect.disabled = true;
        submitBtn.querySelector('.btn-text').style.opacity = '0.5';
    } else {
        spinner.style.display = 'none';
        submitBtn.disabled = false;
        formatSelect.disabled = false;
        engineSelect.disabled=false;
        submitBtn.querySelector('.btn-text').style.opacity = '1';
    }
}


function displayRender(url) {

    // determine type of url based on mime type or file extension
    let format = formatSelect.value;

    if (format === "png"){
        previewDiv.innerHTML = `<img src="${url}" alt="Rendered output" style="max-width: 100%;">`;
    }
    
    if (format === "pdf"){
        previewDiv.innerHTML = `<object data="${url}" type="application/pdf" width="100%" height="100%" style="min-height: 500px; height: stretch">
            <p>Your browser cannot display PDFs. <a href="${url}">Download</a></p>
        </object>`;
    }
}

function clearPreview() {
    previewDiv.innerHTML = '<div>Rendering… please wait</div>';
}

function errorPreview(errmsg) {
    previewDiv.innerHTML = `<div style="color: red;">Render failed: ${errmsg}</div>`;

}



// // Poll /result/{jobID} until completed or failed
// async function pollResult(jobID, format) {
//     const maxAttempts = 15;
//     let attempts = 0;

//     const poll = async () => {
//         attempts++;
//         try {
//             const resp = await fetch(`/result/${jobID}`);
//             if (resp.status === 200) {
//                 const data = await resp.json();
//                 // Success – we have a presigned URL
//                 if (data.url) {
//                     setStatus('Rendering complete!');
//                     if (format === 'png') {
//                         displayPNG(data.url);
//                     } else {
//                         displayPDF(data.url);
//                     }
//                     setLoading(false);
//                     return;
//                 } else {
//                     throw new Error('No URL in response');
//                 }
//             } else if (resp.status === 202) {
//                 // Still processing
//                 setStatus(`Rendering in progress...`);
//                 if (attempts < maxAttempts) {
//                     setTimeout(poll, 1000); // poll every 1 seconds
//                 } else {
//                     throw new Error('Timeout waiting for render result');
//                 }
//                 return;
//             } else {
//                 // Other error (4xx, 5xx)
//                 const errorText = await resp.text();
//                 throw new Error(`Server error: ${resp.status} - ${errorText}`);
//             }
//         } catch (err) {
//             setStatus(`Failed: ${err.message}`, true);
//             setLoading(false);
//             errorPreview()
//         }
//     };

//     poll();
// }


// Submit handler
form.addEventListener('submit', async (e) => {
    e.preventDefault();

    // Clear previous preview and set loading state
    clearPreview()
    setLoading(true);
    setStatus('Submitting job...');

    const formData = new FormData();
    formData.append('engine', engineSelect.value);
    formData.append('source', codeEditor.getValue());
    formData.append('format', formatSelect.value);

    try {
        const response = await fetch('/api/jobs', {
            method: 'POST',
            body: formData
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Server returned ${response.status}: ${errorText}`);
        }

        const data = await response.json();
        const jobID = data.job_id;
        if (!jobID) {
            throw new Error('No job ID returned from server');
        }

        setStatus(`Job submitted. Waiting for render...`);
        // Start polling with the selected format
        streamJob(jobID)
        // pollResult(jobID, formatSelect.value);
    } catch (err) {
        setStatus(`Submission failed: ${err.message}`, true);
        setLoading(false);
        errorPreview(err.message)
    }
});

function streamJob(jobID) {
    const es = new EventSource(`/api/jobs/${jobID}/stream`);


    es.addEventListener('info', (event) => {
        const data = JSON.parse(event.data);

        switch (data.status) {
            case JobStatusQueued:
                setStatus('Queued...')
                break;
            case JobStatusProcessing:
                setStatus('Processing...')
                break;
            case JobStatusUploading:
                setStatus('Uploading...')
                break;
            case JobStatusFailed:
                setStatus(`Error: ${data.error}`, true)
                setLoading(false);
                errorPreview(data.error)
                es.close()
                break;
            case JobStatusCompleted:
                setStatus('Done');
                displayRender(data.result_url)
                setLoading(false);
                es.close()
                break;
        }
    })
    es.onerror =() => {
        if (es.readyState === EventSource.CLOSED){
            setStatus("Connection lost. ???", true)
        }
    }
}
