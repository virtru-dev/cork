import grpc

import cork_pb2
import cork_pb2_grpc


def main():
    channel = grpc.insecure_channel('localhost:11900')
    stub = cork_pb2_grpc.CorkTypeServiceStub(channel)

    res = stub.StepList(cork_pb2.StepListRequest())
    print res

    stub.Kill(cork_pb2.KillRequest())

    return stub


if __name__ == "__main__":
    main()
