FROM python:3.9-slim
ENV TZ=Asia/Shanghai PYTHONUNBUFFERED=1 LANG=C.UTF-8
WORKDIR /app
RUN pip install --no-cache-dir flask requests pycryptodome -i https://pypi.tuna.tsinghua.edu.cn/simple
RUN mkdir -p /app/data
COPY app.py .
EXPOSE 5080
CMD ["python", "app.py"]
