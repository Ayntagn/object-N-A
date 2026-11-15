
  let snippetHoverCard;
  let currentSnippetLink;
  let snippetHideTimer;

  function hideSnippetPreview() {
    clearTimeout(snippetHideTimer);
    currentSnippetLink = null;
    if (snippetHoverCard) {
      snippetHoverCard.hidden = true;
      snippetHoverCard.replaceChildren();
    }
  }

  function showSnippetPreview(link) {
    clearTimeout(snippetHideTimer);
    if (currentSnippetLink === link) return;
    currentSnippetLink = link;
    if (!snippetHoverCard) {
      snippetHoverCard = document.createElement('div');
      snippetHoverCard.className = 'snippet-hover-card';
      snippetHoverCard.id = 'cw-snippet-hover';
      snippetHoverCard.setAttribute('role', 'tooltip');
      snippetHoverCard.addEventListener('pointerenter', () => clearTimeout(snippetHideTimer));
      snippetHoverCard.addEventListener('pointerleave', hideSnippetPreview);
      document.body.append(snippetHoverCard);
    }
    const template = document.querySelector(`template[data-snippet-template="${link.dataset.snippetCard}"]`);
    snippetHoverCard.replaceChildren(template?.content.cloneNode(true) ?? 'Not found');
    snippetHoverCard.hidden = false;
    snippetHoverCard.scrollTop = 0;
    const anchor = link.getBoundingClientRect();
    const top = anchor.bottom + 408 > innerHeight ? anchor.bottom - 410 : anchor.bottom + 8;
    snippetHoverCard.style.top = Math.max(10, top) + 'px';
    snippetHoverCard.style.left = Math.max(10, Math.min(anchor.left, innerWidth - 440)) + 'px';
  }

  document.addEventListener('pointerover', event => {
    const link = event.target.closest?.('a.snippet-link');
    if (link) showSnippetPreview(link);
  });
  document.addEventListener('pointerout', event => {
    if (currentSnippetLink?.contains(event.target) && !currentSnippetLink.contains(event.relatedTarget)) {
      snippetHideTimer = setTimeout(hideSnippetPreview, 300);
    }
  });
  document.addEventListener('focusin', event => {
    const link = event.target.closest?.('a.snippet-link');
    if (link) showSnippetPreview(link);
  });
  document.addEventListener('focusout', event => {
    if (event.target === currentSnippetLink) hideSnippetPreview();
  });
