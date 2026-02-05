import os, sys, json, time, hashlib, base64, struct, socket, requests, datetime, traceback
from flask import Flask, request, make_response, jsonify, render_template_string
from Crypto.Cipher import AES

app = Flask(__name__)
CONFIG_FILE = '/app/data/config.json'

def load_config():
    if not os.path.exists(CONFIG_FILE): return {}
    try:
        with open(CONFIG_FILE, 'r', encoding='utf-8') as f: return json.load(f)
    except: return {}

def save_config(data):
    with open(CONFIG_FILE, 'w', encoding='utf-8') as f:
        json.dump(data, f, indent=4)

HTML_TEMPLATE = '''
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>群晖 SynologyWebhook</title>
    <style>
        :root { --primary: #0086f6; --text: #333; }
        body { 
            font-family: -apple-system, "Microsoft YaHei", sans-serif; 
            background: url('https://t.alcy.cc/fj') no-repeat center center fixed; 
            background-size: cover;
            display: flex; justify-content: center; padding: 20px; color: var(--text); margin: 0; min-height: 100vh; align-items: center;
        }
        .container { 
            background: rgba(255, 255, 255, 0.85); 
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
            padding: 30px; width: 100%; max-width: 480px; border-radius: 16px; 
            box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.37);
            border: 1px solid rgba(255, 255, 255, 0.18);
        }
        .header { text-align: center; margin-bottom: 25px; border-bottom: 1px solid rgba(0,0,0,0.1); padding-bottom: 15px; }
        h2 { margin: 0; font-size: 22px; color: #1d2c40; text-shadow: 0 1px 2px rgba(0,0,0,0.1); }
        .sub-title { font-size: 12px; color: #666; margin-top: 5px; }
        .form-group { margin-bottom: 18px; }
        label { display: block; margin-bottom: 6px; font-weight: 600; font-size: 13px; color: #444; }
        input { width: 100%; padding: 10px; border: 1px solid #ccc; border-radius: 8px; box-sizing: border-box; font-size: 14px; outline: none; transition: all 0.2s; background: rgba(255,255,255,0.9); }
        input:focus { border-color: var(--primary); box-shadow: 0 0 5px rgba(0,134,246,0.3); }
        button { width: 100%; padding: 12px; background: var(--primary); color: white; border: none; border-radius: 8px; font-size: 15px; font-weight: 600; cursor: pointer; margin-top: 10px; transition: opacity 0.2s; box-shadow: 0 4px 6px rgba(0,134,246,0.3); }
        button:hover { opacity: 0.9; transform: translateY(-1px); }
        .alert { padding: 12px; border-radius: 6px; margin-bottom: 20px; text-align: center; font-size: 13px; background: rgba(209, 250, 229, 0.9); color: #047857; border: 1px solid #a7f3d0; }
        .note { font-size: 12px; color: #777; margin-top: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>群晖 SynologyWebhook</h2>
            <div class="sub-title">企业微信消息推送代理</div>
        </div>
        {% if message %}<div class="alert">{{ message }}</div>{% endif %}
        <form method="POST">
            <div class="form-group"><label>CorpID (企业ID)</label><input type="text" name="corp_id" value="{{ config.get('corp_id', '') }}" required placeholder="ww开头"></div>
            <div class="form-group"><label>Secret (应用密钥)</label><input type="text" name="corp_secret" value="{{ config.get('corp_secret', '') }}" required></div>
            <div class="form-group"><label>Agent ID (应用ID)</label><input type="text" name="agent_id" value="{{ config.get('agent_id', '') }}" required></div>
            <div class="form-group"><label>Token</label><input type="text" name="token" value="{{ config.get('token', '') }}" required></div>
            <div class="form-group"><label>EncodingAESKey</label><input type="text" name="aes_key" value="{{ config.get('aes_key', '') }}" required></div>
            <div class="form-group">
                <label>☁️ 远程 API 代理 (选填)</label>
                <input type="text" name="api_host" value="{{ config.get('api_host', '') }}" placeholder="例如: http://1.2.3.4:5080">
                <div class="note">留空则直接请求腾讯服务器，填入则走云端代理</div>
            </div>
            <div class="form-group">
                <label>跳转链接 (Card URL)</label>
                <input type="text" name="card_url" value="{{ config.get('card_url', '') }}" placeholder="点击卡片跳转的 NAS 地址">
            </div>
            <button type="submit">保存配置</button>
        </form>
    </div>
</body>
</html>
'''

class WXBizMsgCrypt:
    def __init__(self, sToken, sEncodingAesKey, sReceiveId):
        try:
            self.key = base64.b64decode(sEncodingAesKey + "=")
            assert len(self.key) == 32
        except: raise Exception("AESKey 格式错误")
        self.token = sToken
        self.corpid = sReceiveId
        self.mode = AES.MODE_CBC

    def VerifyURL(self, sMsgSignature, sTimeStamp, sNonce, sEchoStr):
        sort_list = [self.token, sTimeStamp, sNonce, sEchoStr]
        sort_list.sort()
        sha = hashlib.sha1("".join(sort_list).encode('utf-8')).hexdigest()
        if sha != sMsgSignature: return None
        crypt_data = base64.b64decode(sEchoStr)
        cipher = AES.new(self.key, self.mode, self.key[:16])
        plain_text = cipher.decrypt(crypt_data)
        content = plain_text[:-plain_text[-1]]
        xml_len = socket.ntohl(struct.unpack("I", content[16:20])[0])
        return content[20 : 20 + xml_len]

@app.route('/', methods=['GET', 'POST'])
def index():
    msg = ''
    if request.method == 'POST':
        cfg = {
            'corp_id': request.form.get('corp_id', '').strip(),
            'corp_secret': request.form.get('corp_secret', '').strip(),
            'agent_id': request.form.get('agent_id', '').strip(),
            'token': request.form.get('token', '').strip(),
            'aes_key': request.form.get('aes_key', '').strip(),
            'api_host': request.form.get('api_host', '').strip().rstrip('/'),
            'card_url': request.form.get('card_url', '').strip()
        }
        save_config(cfg)
        msg = '✅ 配置已保存并生效'
        return render_template_string(HTML_TEMPLATE, config=cfg, message=msg)
    return render_template_string(HTML_TEMPLATE, config=load_config())

@app.route('/webhook', methods=['GET', 'POST'])
def webhook():
    cfg = load_config()
    if not cfg.get('corp_id'): return jsonify({'err': 'Not configured'}), 503
    if request.method == 'GET':
        try:
            wxcpt = WXBizMsgCrypt(cfg['token'], cfg['aes_key'], cfg['corp_id'])
            ret = wxcpt.VerifyURL(request.args.get('msg_signature'), request.args.get('timestamp'), request.args.get('nonce'), request.args.get('echostr'))
            if ret:
                resp = make_response(ret)
                resp.headers['Content-Type'] = 'text/plain'
                return resp
        except: pass
        return 'Fail', 403
    content = ''
    try:
        data = request.json or (json.loads(request.form['payload']) if request.form.get('payload') else request.form.to_dict())
        content = data.get('text') or data.get('message') or data.get('content') or data.get('description')
    except: pass
    if not content:
        raw = request.data.decode('utf-8')
        if raw and not raw.strip().startswith('{'): content = raw
    if content:
        return send_to_wechat(cfg, content)
    return 'ok', 200

def send_to_wechat(cfg, text):
    try:
        api_host = cfg.get('api_host') or 'https://qyapi.weixin.qq.com'
        token_url = f"{api_host}/cgi-bin/gettoken?corpid={cfg['corp_id']}&corpsecret={cfg['corp_secret']}"
        r = requests.get(token_url, timeout=5).json()
        token = r.get('access_token')
        if not token: return jsonify(r)
        send_url = f"{api_host}/cgi-bin/message/send?access_token={token}"
        now = datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        text = str(text).replace('\\n', '<br>').replace('[Autunn]', '').strip()
        pic_url = f"https://t.alcy.cc/fj?v={int(time.time())}"
        payload = {
            "touser": "@all", "msgtype": "news", "agentid": cfg['agent_id'],
            "news": {
                "articles": [
                    {
                        "title": "🔔 NAS 通知",
                        "description": text,
                        "url": cfg.get('card_url') or "https://work.weixin.qq.com",
                        "picurl": pic_url
                    }
                ]
            }
        }
        res = requests.post(send_url, json=payload, timeout=5).json()
        return jsonify(res)
    except Exception as e:
        return jsonify({'error': str(e)})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5080)
