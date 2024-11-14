import json
import sys

filename = "ipTable.json"

port_start = 52000

num = int(sys.argv[1])


def create_data(num):
    inter = {}
    for i in range(num):
        inter[i] = "127.0.0.1:" + str(port_start + i)
    data = {}
    for i in range(num):
        data[i] = inter
    # add supervisor
    data[2147483647] = {0: "127.0.0.1:38800"}
    return data


def create_json(num):
    d = create_data(num)
    with open("ipTable.json", "w", encoding="utf8") as f:
        json.dump(d, f, indent=4)


create_json(num)
