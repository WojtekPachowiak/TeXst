// render.js – Vanilla JS polling for async rendering

const form = document.getElementById('renderForm');
const sourceTextarea = document.getElementById('source');
const engineSelect = document.getElementById('engine');
const formatSelect = document.getElementById('format');
const submitBtn = document.getElementById('submitBtn');
const spinner = document.getElementById('spinner');
const statusDiv = document.getElementById('status');
const previewDiv = document.getElementById('preview');

// Helper: show status message (can be error or info)
function setStatus(message, isError = false) {
    statusDiv.innerHTML = message;
    statusDiv.style.color = isError ? '#b91c1c' : '#475569';
}

// Helper: show/hide spinner
function setLoading(loading) {
    if (loading) {
        spinner.style.display = 'inline-block';
        submitBtn.disabled = true;
        submitBtn.querySelector('.btn-text').style.opacity = '0.5';
    } else {
        spinner.style.display = 'none';
        submitBtn.disabled = false;
        submitBtn.querySelector('.btn-text').style.opacity = '1';
    }
}

// Display PDF (using <object>)
function displayPDF(url) {
    previewDiv.innerHTML = `<object data="${url}" type="application/pdf" width="100%" height="100%" style="min-height: 500px;">
        <p>Your browser cannot display PDFs. <a href="${url}">Download</a></p>
    </object>`;
}

// Display PNG (using <img>)
function displayPNG(url) {
    previewDiv.innerHTML = `<img src="${url}" alt="Rendered output" style="max-width: 100%; border: 1px solid #e2e8f0;">`;
}

// Poll /result/{jobID} until completed or failed
async function pollResult(jobID, format) {
    const maxAttempts = 15; 
    let attempts = 0;

    const poll = async () => {
        attempts++;
        try {
            const resp = await fetch(`/result/${jobID}`);
            if (resp.status === 200) {
                const data = await resp.json();
                // Success – we have a presigned URL
                if (data.url) {
                    setStatus('Rendering complete!');
                    if (format === 'png') {
                        displayPNG(data.url);
                    } else {
                        displayPDF(data.url);
                    }
                    setLoading(false);
                    return;
                } else {
                    throw new Error('No URL in response');
                }
            } else if (resp.status === 202) {
                // Still processing
                setStatus(`Rendering in progress...`);
                if (attempts < maxAttempts) {
                    setTimeout(poll, 1000); // poll every 1 seconds
                } else {
                    throw new Error('Timeout waiting for render result');
                }
                return;
            } else {
                // Other error (4xx, 5xx)
                const errorText = await resp.text();
                throw new Error(`Server error: ${resp.status} - ${errorText}`);
            }
        } catch (err) {
            setStatus(`Failed: ${err.message}`, true);
            setLoading(false);
            previewDiv.innerHTML = `<div style="color: #b91c1c;">Render failed: ${err.message}</div>`;
        }
    };

    poll();
}



// Submit handler
form.addEventListener('submit', async (e) => {
    e.preventDefault();

    // Clear previous preview and set loading state
    previewDiv.innerHTML = '<div style="color: #64748b;">Rendering… please wait</div>';
    setLoading(true);
    setStatus('Submitting job...');

    const formData = new FormData();
    formData.append('engine', engineSelect.value);
    formData.append('source', sourceTextarea.value);
    formData.append('format', formatSelect.value);

    try {
        const response = await fetch('/render', {
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

        setStatus(`Job submitted (ID: ${jobID}). Waiting for render...`);
        // Start polling with the selected format
        pollResult(jobID, formatSelect.value);
    } catch (err) {
        setStatus(`Submission failed: ${err.message}`, true);
        setLoading(false);
        previewDiv.innerHTML = `<div style="color: #b91c1c;">Error: ${err.message}</div>`;
    }
});