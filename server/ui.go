package server

import (
	"net/http"
	"strings"
)

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	token, hasToken := s.cfg.Server.BearerToken()
	if !hasToken {
		_, _ = w.Write([]byte(indexHTML))
		return
	}
	// Inject the bearer token so the in-page JS can authenticate API calls.
	// Safe because the server only binds to loopback.
	snippet := `<script>window.__ginko_token="` + token + `";</script>`
	_, _ = w.Write([]byte(strings.Replace(indexHTML, "</head>", snippet+"</head>", 1)))
}

const indexHTML = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>llm-memory</title>
<style>
:root{color-scheme:dark;--bg:#0b0f14;--panel:#111821;--muted:#8ba0b3;--txt:#e6edf3;--line:#243244;--accent:#7ee787;--warn:#d29922;--bad:#ff7b72;--blue:#1f6feb}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--txt);font:13px/1.45 system-ui,Segoe UI,Roboto,Arial}header{padding:14px 18px;border-bottom:1px solid var(--line);display:flex;gap:16px;align-items:center;justify-content:space-between}h1{font-size:18px;margin:0}.wrap{padding:14px}.card{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:14px;margin-bottom:12px}.toolbar{display:grid;grid-template-columns:1.4fr repeat(4,minmax(120px,.4fr)) auto;gap:8px;align-items:end}label{display:block;margin:0 0 4px;color:var(--muted);font-size:12px}input,textarea,select{width:100%;background:#0d141c;color:var(--txt);border:1px solid var(--line);border-radius:8px;padding:8px}textarea{min-height:120px}button{background:var(--blue);color:white;border:0;border-radius:8px;padding:9px 12px;cursor:pointer}button.secondary{background:#30363d}button.danger{background:#9b2428}.row{display:flex;gap:8px}.row>*{flex:1}.muted{color:var(--muted)}.err{color:var(--bad)}.warn{color:var(--warn)}code{color:var(--accent)}table{width:100%;border-collapse:collapse}th,td{border-bottom:1px solid var(--line);padding:8px 7px;text-align:left;vertical-align:top}th{color:var(--muted);font-size:12px;position:sticky;top:0;background:var(--panel);z-index:1}tbody tr:hover{background:#0d141c}.content{max-width:720px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.tag{display:inline-block;color:#111;background:var(--accent);padding:1px 5px;border-radius:999px;margin-right:4px;font-size:11px}.pill{border:1px solid var(--line);border-radius:999px;padding:2px 7px;color:var(--muted)}dialog{background:var(--panel);color:var(--txt);border:1px solid var(--line);border-radius:14px;max-width:760px;width:92vw}dialog::backdrop{background:rgba(0,0,0,.55)}.actions{white-space:nowrap}.stats{display:flex;gap:10px;flex-wrap:wrap}.stat{background:#0d141c;border:1px solid var(--line);border-radius:10px;padding:8px 10px}@media(max-width:950px){.toolbar{grid-template-columns:1fr 1fr}.content{white-space:normal}.hide-sm{display:none}}
</style>
</head>
<body>
<header><h1>☣️ llm-memory</h1><div class="muted" id="cfg">carregando config...</div></header>
<div class="wrap">
  <section class="card">
    <div class="toolbar">
      <div><label>search</label><input id="q" placeholder="texto FTS: respostas diretas, SQLite, k8s..." onkeydown="if(event.key==='Enter') loadTable()"/></div>
      <div><label>type</label><select id="typeFilter"><option value="">any</option><option>preference</option><option>fact</option><option>decision</option><option>task</option><option>note</option><option>relationship</option></select></div>
      <div><label>scope</label><select id="scopeFilter"><option value="">any</option><option>global</option><option>project</option><option>session</option><option>private</option></select></div>
      <div><label>subject</label><input id="subjectFilter" placeholder="botmaster" /></div>
      <div><label>conf</label><select id="confFilter"><option value="0">all</option><option value="0.5" selected>&gt;= .50</option><option value="0.8">&gt;= .80</option></select></div>
      <div><button onclick="loadTable()">Atualizar</button></div>
    </div>
  </section>

  <section class="card">
    <div class="row" style="align-items:center;justify-content:space-between">
      <div class="stats" id="stats"></div>
      <div><button onclick="newMemory()">Nova memória</button> <button class="secondary" onclick="loadPending()">Pendentes</button> <button class="secondary" onclick="loadSupersessions()">Timeline</button> <button class="secondary" onclick="loadDocuments()">Docs RAG</button> <button class="secondary" onclick="loadIngestionRuns()">Ingestões</button> <button class="secondary" onclick="loadEvents()">Eventos</button> <button class="secondary" onclick="loadSettings()">Config</button></div>
    </div>
  </section>

  <section class="card">
    <label>RAG ingest path</label>
    <div class="row" style="align-items:end">
      <div><input id="ingestPath" placeholder="/path/file.md ou /path/pasta" /></div>
      <div style="flex:0 0 100px"><button class="secondary" onclick="openBrowser()">Browse…</button></div>
      <div style="flex:0 0 160px"><label><input id="ingestRecursive" type="checkbox" checked style="width:auto"/> recursivo</label></div>
      <div style="flex:0 0 120px"><button onclick="ingestPath()">Ingerir</button></div>
    </div>
    <p class="muted" id="ingestMsg">Texto/markdown/html/json/csv/tex entram nativo; PDF/DOCX/PPTX/XLSX/imagens usam Docling CLI se instalado.</p>
  </section>

<dialog id="browser" style="min-width:520px">
  <div class="row" style="align-items:center;justify-content:space-between;margin-bottom:8px">
    <div><b>Selecionar path</b></div>
    <button class="secondary" onclick="document.getElementById('browser').close()">✕</button>
  </div>
  <div class="row" style="align-items:center;gap:6px;margin-bottom:8px">
    <code id="browserPath" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;color:var(--muted)"></code>
    <button class="secondary" onclick="browserNav(document.getElementById('browserParent').value)">↑ Up</button>
    <input type="hidden" id="browserParent"/>
  </div>
  <div id="browserList" style="max-height:360px;overflow-y:auto;border:1px solid var(--line);border-radius:8px"></div>
  <div class="row" style="margin-top:10px">
    <button onclick="browserSelect(document.getElementById('browserPath').textContent)">Selecionar pasta atual</button>
    <button class="secondary" onclick="document.getElementById('browser').close()">Cancelar</button>
  </div>
</dialog>

  <main class="card" style="overflow:auto;max-height:70vh">
    <table>
      <thead><tr><th>type</th><th>subject</th><th>content</th><th>conf</th><th>used</th><th class="hide-sm">updated</th><th>tags</th><th></th></tr></thead>
      <tbody id="rows"><tr><td colspan="8" class="muted">carregando...</td></tr></tbody>
    </table>
  </main>
</div>

<dialog id="editor">
  <h2 id="editorTitle">Memória</h2>
  <input id="id" placeholder="id opcional" />
  <div class="row"><div><label>type</label><select id="type"><option>preference</option><option>fact</option><option>decision</option><option>task</option><option>note</option><option>relationship</option></select></div><div><label>scope</label><select id="scope"><option>global</option><option>project</option><option>session</option><option>private</option></select></div></div>
  <label>subject</label><input id="subject" value="botmaster" />
  <label>content</label><textarea id="content"></textarea>
  <div class="row"><div><label>source.kind</label><input id="sourceKind" value="gui" /></div><div><label>source.ref</label><input id="sourceRef" value="local" /></div></div>
  <div class="row"><div><label>confidence</label><input id="confidence" type="number" min="0" max="1" step="0.01" value="0.90" /></div><div><label>tags, vírgula</label><input id="tags" placeholder="style, preference" /></div></div>
  <p class="row"><button onclick="saveMemory()">Salvar</button><button class="secondary" onclick="document.getElementById('editor').close()">Cancelar</button></p>
  <p id="formMsg" class="muted"></p>
</dialog>

<script>
let lastRows=[];
async function api(path,opt={}){const h={'content-type':'application/json'};if(window.__ginko_token)h['Authorization']='Bearer '+window.__ginko_token;const r=await fetch(path,{headers:h,...opt});const j=await r.json().catch(()=>null);if(!r.ok)throw new Error((j&&j.error)||r.statusText);return j}
function val(id){return document.getElementById(id).value.trim()}
function qs(){const p=new URLSearchParams();if(val('q'))p.set('q',val('q'));if(val('subjectFilter'))p.set('subject',val('subjectFilter'));if(val('typeFilter'))p.set('type',val('typeFilter'));if(val('scopeFilter'))p.set('scope',val('scopeFilter'));p.set('limit','300');return p.toString()}
async function loadTable(){try{const minConf=parseFloat(val('confFilter')||'0');const rows=await api('/api/usage?'+qs());lastRows=rows.filter(r=>(r.memory.confidence||0)>=minConf);renderRows(lastRows);renderStats(lastRows)}catch(e){document.getElementById('rows').innerHTML='<tr><td colspan="8" class="err">'+esc(e.message)+'</td></tr>'}}
function renderStats(rows){const total=rows.length, zombies=rows.filter(r=>r.usage.context_uses===0).length, hot=rows.filter(r=>r.usage.context_uses>=5).length, low=rows.filter(r=>r.memory.confidence<.5).length;document.getElementById('stats').innerHTML=[['total',total],['zombies',zombies],['hot',hot],['low conf',low]].map(x=>'<span class="stat"><b>'+x[1]+'</b> <span class="muted">'+x[0]+'</span></span>').join('')}
function renderRows(rows){const tbody=document.getElementById('rows');if(!rows.length){tbody.innerHTML='<tr><td colspan="8" class="muted">sem resultados</td></tr>';return}tbody.innerHTML=rows.map(r=>{const m=r.memory,u=r.usage;const zombie=u.context_uses===0?' <span class="warn">⚠</span>':'';return '<tr><td><span class="pill">'+esc(m.type)+'</span></td><td>'+esc(m.subject)+'</td><td class="content" title="'+esc(m.content)+'">'+esc(m.content)+'</td><td>'+Number(m.confidence).toFixed(2)+'</td><td><b>'+u.context_uses+'x</b>'+zombie+'<div class="muted">+'+u.useful_votes+' / -'+u.useless_votes+'</div></td><td class="hide-sm">'+fmtDate(m.updated_at)+'</td><td>'+(m.tags||[]).map(t=>'<span class="tag">'+esc(t)+'</span>').join('')+'</td><td class="actions"><button class="secondary" onclick=\'editMemory('+JSON.stringify(m).replaceAll("'","&#39;")+')\'>Editar</button> <button class="danger" onclick="forget(\''+m.id+'\')">Forget</button></td></tr>'}).join('')}
function memoryFromForm(){return {id:val('id')||undefined,type:val('type'),subject:val('subject'),content:val('content'),source:{kind:val('sourceKind'),ref:val('sourceRef')},scope:val('scope'),confidence:parseFloat(val('confidence')||'0.9'),tags:val('tags')?val('tags').split(',').map(s=>s.trim()).filter(Boolean):[],embedding_refs:{}}}
async function saveMemory(){try{const m=await api('/api/memories',{method:'POST',body:JSON.stringify(memoryFromForm())});document.getElementById('formMsg').textContent='salvo: '+m.id;document.getElementById('editor').close();loadTable()}catch(e){document.getElementById('formMsg').innerHTML='<span class="err">'+esc(e.message)+'</span>'}}
function newMemory(){clearForm();document.getElementById('editorTitle').textContent='Nova memória';document.getElementById('editor').showModal()}
function clearForm(){for(const id of ['id','content','tags'])document.getElementById(id).value='';document.getElementById('type').value='preference';document.getElementById('scope').value='global';document.getElementById('subject').value=val('subjectFilter')||'botmaster';document.getElementById('sourceKind').value='gui';document.getElementById('sourceRef').value='local';document.getElementById('confidence').value='0.90';document.getElementById('formMsg').textContent=''}
function editMemory(m){document.getElementById('editorTitle').textContent='Editar memória';document.getElementById('id').value=m.id;document.getElementById('type').value=m.type;document.getElementById('scope').value=m.scope;document.getElementById('subject').value=m.subject;document.getElementById('content').value=m.content;document.getElementById('sourceKind').value=m.source.kind;document.getElementById('sourceRef').value=m.source.ref;document.getElementById('confidence').value=m.confidence;document.getElementById('tags').value=(m.tags||[]).join(', ');document.getElementById('formMsg').textContent='';document.getElementById('editor').showModal()}
async function forget(id){if(!confirm('Forget '+id+'?'))return;await api('/api/memories/'+id,{method:'DELETE'});loadTable()}
async function ingestPath(){try{const path=val('ingestPath');if(!path)throw new Error('path obrigatório');document.getElementById('ingestMsg').textContent='ingerindo...';const out=await api('/api/ingest',{method:'POST',body:JSON.stringify({path,recursive:document.getElementById('ingestRecursive').checked})});document.getElementById('ingestMsg').textContent=out.run.id+' '+out.run.status+': '+out.documents.length+' docs, '+out.chunks.length+' chunks, '+out.skipped.length+' skipped';loadDocuments()}catch(e){document.getElementById('ingestMsg').innerHTML='<span class="err">'+esc(e.message)+'</span>'}}
async function loadDocuments(){const docs=await api('/api/documents?limit=200');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Documentos RAG</h3>'+docs.map(d=>'<div style="border-bottom:1px solid var(--line);padding:8px"><b>'+esc(d.title)+'</b> <code>'+esc(d.id)+'</code><div>'+esc(d.path)+'</div><span class="muted">run '+esc(d.ingestion_run_id||'-')+' · '+esc(d.source_kind)+' · sha256 '+esc((d.sha256||'').slice(0,16))+'… · '+fmtDate(d.created_at)+'</span><div style="margin-top:6px"><button class="secondary" onclick=\'loadChunks("'+esc(d.id)+'")\'>ver chunks</button> <button class="secondary" onclick=\'suggestFromDocument("'+esc(d.id)+'")\'>sugerir memórias</button></div></div>').join('')+'</td></tr>'}
async function loadChunks(documentId){const rows=await api('/api/chunks/search',{method:'POST',body:JSON.stringify({document_id:documentId,limit:100})});document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Chunks '+esc(documentId)+'</h3>'+rows.map(r=>'<div style="border-bottom:1px solid var(--line);padding:8px"><b>#'+r.chunk.ordinal+'</b> <code>'+esc(r.chunk.id)+'</code><pre>'+esc(r.chunk.content)+'</pre><span class="muted">doc '+esc(r.document.id)+' · run '+esc(r.document.ingestion_run_id||'-')+' · tokens '+r.chunk.token_count+'</span></div>').join('')+'</td></tr>'}
async function suggestFromDocument(documentId){const subject=val('subjectFilter')||'';const out=await api('/api/documents/'+documentId+'/suggest',{method:'POST',body:JSON.stringify({subject,scope:'project',limit:20})});document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Candidatos de memória '+esc(documentId)+'</h3>'+out.candidates.map(c=>'<div style="border-bottom:1px solid var(--line);padding:8px"><b>'+esc(c.memory.type)+'</b> <code>'+esc(c.memory.source.ref)+'</code><pre>'+esc(c.memory.content)+'</pre><span class="muted">conf '+Number(c.memory.confidence).toFixed(2)+' · '+esc(c.reason)+'</span></div>').join('')+'</td></tr>'}
async function loadIngestionRuns(){const runs=await api('/api/ingestion-runs?limit=100');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Ingestões RAG</h3>'+runs.map(r=>'<div style="border-bottom:1px solid var(--line);padding:8px"><b>'+esc(r.status)+'</b> <code>'+esc(r.id)+'</code><div>'+esc(r.source_path)+'</div><span class="muted">parser '+esc(r.parser)+' · recursive '+r.recursive+' · files '+r.files_seen+' · docs '+r.documents_created+' · chunks '+r.chunks_created+' · '+fmtDate(r.created_at)+'</span>'+(r.error?'<pre class="err">'+esc(r.error)+'</pre>':'')+'</div>').join('')+'</td></tr>'}
async function loadEvents(){const items=await api('/api/events?limit=100');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Eventos</h3>'+items.map(e=>'<div style="border-bottom:1px solid var(--line);padding:8px"><b>'+esc(e.kind)+'</b> <code>'+esc(e.id)+'</code><pre>'+esc(e.payload)+'</pre><span class="muted">'+esc(e.source.kind)+':'+esc(e.source.ref)+' · '+fmtDate(e.created_at)+'</span></div>').join('')+'</td></tr>'}
function fmtDate(s){return s?new Date(s).toLocaleString():''}
function esc(s){return String(s||'').replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]))}
async function openBrowser(){await browserNav('');document.getElementById('browser').showModal()}
async function browserNav(path){const data=await api('/api/browse'+(path?'?path='+encodeURIComponent(path):''));document.getElementById('browserPath').textContent=data.path;document.getElementById('browserParent').value=data.parent;const list=document.getElementById('browserList');list.innerHTML=data.entries.map(e=>'<div style="padding:7px 10px;border-bottom:1px solid var(--line);cursor:pointer;display:flex;align-items:center;gap:8px" onclick=\''+( e.is_dir?'browserNav("'+e.path+'")':'browserSelect("'+e.path+'")'  )+'\''+'><span style="color:var(--muted)">'+(e.is_dir?'📁':'📄')+'</span><span>'+esc(e.name)+'</span>'+(e.is_dir?'':'<span class="muted" style="margin-left:auto;font-size:11px">selecionar</span>')+'</div>').join('')||'<div class="muted" style="padding:10px">pasta vazia</div>'}
function browserSelect(path){document.getElementById('ingestPath').value=path;document.getElementById('browser').close()}
async function loadSupersessions(){const items=await api('/api/analytics/supersessions?limit=50');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Timeline de Supersession</h3>'+(items&&items.length?'<table style="width:100%"><tr><th>Quando</th><th>Subject</th><th>Tipo</th><th>Antigo</th><th>Novo</th></tr>'+items.map(e=>'<tr><td>'+fmtDate(e.created_at)+'</td><td>'+esc(e.subject)+'</td><td><span class="pill">'+esc(e.old_type)+'</span></td><td class="muted" style="font-size:11px">'+esc(e.old_id.slice(0,16))+'…</td><td class="muted" style="font-size:11px">'+esc(e.new_id.slice(0,16))+'…</td></tr>').join('')+'</table>':'<div class="muted" style="padding:10px">Nenhuma supersession registrada.</div>')+'</td></tr>'}
async function loadPending(){const rows=await api('/api/memories?status=pending&limit=200');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Memórias pendentes de aprovação</h3>'+(rows.length===0?'<div class="muted" style="padding:10px">Nenhuma memória pendente.</div>':rows.map(m=>'<div style="border-bottom:1px solid var(--line);padding:10px"><div class="row" style="align-items:center;gap:8px"><span class="pill">'+esc(m.type)+'</span><b>'+esc(m.subject)+'</b><span class="muted">conf '+Number(m.confidence).toFixed(2)+'</span></div><pre style="margin:6px 0">'+esc(m.content)+'</pre><div class="row" style="gap:6px"><button onclick=\'approveMemory("'+m.id+'")\'>Aprovar</button><button class="secondary" onclick=\'editMemory('+JSON.stringify(m).replaceAll("\'","&#39;")+')\'>Editar</button><button class="danger" onclick=\'forget("'+m.id+'")\'>Rejeitar</button></div></div>').join(''))+'</td></tr>'}
async function approveMemory(id){try{await api('/api/memories/'+id+'/approve',{method:'POST'});loadPending()}catch(e){alert('Erro: '+e.message)}}
async function loadSettings(){const c=await api('/api/config');document.getElementById('rows').innerHTML='<tr><td colspan="8"><h3>Configuração</h3><form onsubmit="saveSettings(event)"><table style="max-width:640px"><tr><th>Campo</th><th>Valor</th></tr><tr><td>database</td><td><code>'+esc(c.database.path)+'</code> <span class="muted">(somente leitura)</span></td></tr><tr><td>server.addr</td><td><input id="cfg_addr" value="'+esc(c.server.addr)+'" style="width:220px"/></td></tr><tr><td>llm.provider</td><td><input id="cfg_llm_provider" value="'+esc(c.llm.provider)+'" style="width:180px"/></td></tr><tr><td>llm.model</td><td><input id="cfg_llm_model" value="'+esc(c.llm.model||'')+'" style="width:220px"/></td></tr><tr><td>embedding.provider</td><td><input id="cfg_emb_provider" value="'+esc(c.embedding.provider)+'" style="width:180px"/></td></tr><tr><td>embedding.model</td><td><input id="cfg_emb_model" value="'+esc(c.embedding.model||'')+'" style="width:220px"/></td></tr><tr><td>embedding.index</td><td><select id="cfg_emb_index"><option'+(c.embedding.index==="sqlite-fts"?" selected":"")+'>sqlite-fts</option><option'+(c.embedding.index==="vector"?" selected":"")+'>vector</option></select></td></tr></table><div class="row" style="margin-top:10px;gap:8px"><button type="submit">Salvar e reiniciar ginko serve</button></div><p id="cfgMsg" class="muted" style="margin-top:6px"></p></form><p class="muted" style="margin-top:8px;font-size:11px">Salvar escreve ~/.ginko/config.json e reinicia o servidor. Database path só pode ser alterado manualmente.</p></td></tr>'}
async function saveSettings(e){e.preventDefault();const body={server:{addr:document.getElementById('cfg_addr').value},llm:{provider:document.getElementById('cfg_llm_provider').value,model:document.getElementById('cfg_llm_model').value},embedding:{provider:document.getElementById('cfg_emb_provider').value,model:document.getElementById('cfg_emb_model').value,index:document.getElementById('cfg_emb_index').value}};try{await api('/api/config',{method:'POST',body:JSON.stringify(body)});document.getElementById('cfgMsg').textContent='Salvo. Restarting...';setTimeout(()=>location.reload(),1500)}catch(err){document.getElementById('cfgMsg').innerHTML='<span class="err">'+esc(err.message)+'</span>'}}
api('/api/config').then(c=>document.getElementById('cfg').textContent='db '+c.database.path+' · llm '+c.llm.provider+'/'+(c.llm.model||'-')+' · embedding '+c.embedding.provider+'/'+(c.embedding.model||'-'));loadTable();
</script>
</body>
</html>`
