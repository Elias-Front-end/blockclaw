(() => {
  const messagesEl = document.getElementById('cc-messages');
  const form = document.getElementById('cc-form');
  const input = document.getElementById('cc-input');

  function addMessage(sender, text) {
    const msg = document.createElement('div');
    msg.className = 'message ' + (sender === 'user' ? 'user' : 'pico');
    const author = document.createElement('span');
    author.className = 'author';
    author.textContent = sender === 'user' ? 'Voce' : 'PicoClaw';
    const textEl = document.createElement('div');
    textEl.textContent = text;
    msg.appendChild(author);
    msg.appendChild(textEl);
    messagesEl.appendChild(msg);
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  async function sendToBackend(message) {
    // Endpoint to chat with PicoClaw. This is a best-effort default and can be replaced
    // by wiring to a real gateway (e.g. /picoclaw/chat or /gateway/chat).
    const candidates = [
      '/picoclaw/chat',
      '/gateway/chat',
      '/chat',
    ];
    let lastErr = null;
    for (const url of candidates) {
      try {
        const res = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ message })
        });
        if (res.ok) {
          const data = await res.json();
          return data && data.response ? data.response : '';
        } else {
          lastErr = new Error('HTTP ' + res.status);
        }
      } catch (e) {
        lastErr = e;
      }
    }
    throw lastErr || new Error('Unknown error');
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    addMessage('user', text);
    input.value = '';
    addMessage('pico', '...'); // loading placeholder
    try {
      const response = await sendToBackend(text);
      // replace loading placeholder with actual response
      const last = messagesEl.lastChild;
      if (last) last.querySelector('.author').textContent = 'PicoClaw';
      if (response) {
        // replace the loading text node
        const loading = messagesEl.lastChild;
        if (loading) {
          loading.querySelector('div')?.textContent = response;
        }
      } else {
        if (last) {
          const div = last.querySelector('div');
          if (div) div.textContent = '(sem resposta)';
        }
      }
    } catch (err) {
      // show error
      const loading = messagesEl.lastChild;
      if (loading) {
        const div = loading.querySelector('div');
        if (div) div.textContent = 'Erro ao falar com PicoClaw';
      }
      console.error(err);
    }
  });
})();
