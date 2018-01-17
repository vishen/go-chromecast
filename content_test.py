import requests
import binascii
import hashlib
# curl example: curl "localhost:34455?media_file=/home/jonathan/Downloads/jellyfish-25-mbps-hd-hevc.mp4&live_streaming=true" --header "Range: bytes=0-"

def make_request(live_streaming, range_header_value):
    media_file = "/home/jonathan/Downloads/jellyfish-25-mbps-hd-hevc.mp4"
    ls = "false"
    if live_streaming:
        ls = "true"

    res = requests.get(
        "http://localhost:34455?media_file={}&live_streaming={}".format(media_file, ls),
        headers={"Range": range_header_value}
    )

    return res


def h(b):
    hash_object = hashlib.sha1(b)
    return hash_object.hexdigest()
    #return binascii.hexlify(b)

def main():
    r = make_request(False, "bytes=0-")
    r_live = make_request(True, "bytes=0-")

    print(r.headers)
    print(r_live.headers)
    print(h(r.content))
    print(h(r_live.content))
    print()
    print(h(r.content[-20:]))
    print(h(r_live.content[-20:]))

    print(h(r.content) == h(r_live.content))

    print("###################################")

    br = "bytes=32768-"
    r = make_request(False, br)
    r_live = make_request(True, br)
    print(r.headers)
    print(r_live.headers)

    print(h(r.content)[:20])
    print(h(r_live.content)[:20])
    print()
    print(h(r.content)[-20:])
    print(h(r_live.content)[-20:])
    print()
    print(h(r.content) == h(r_live.content))
    print("###################################")

    br = "bytes=32800768-"
    r = make_request(False, br)
    r_live = make_request(True, br)
    print(r.headers)
    print(r_live.headers)

    print(h(r.content)[:20])
    print(h(r_live.content)[:20])
    print()
    print(h(r.content)[-20:])
    print(h(r_live.content)[-20:])
    print()
    print(h(r.content) == h(r_live.content))


if __name__ == "__main__":
    main()
