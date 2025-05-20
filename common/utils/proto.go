package utils

type ProtoConvertible[T any] interface {
	ToProto() *T
}

func ToProtoSlice[T any, P ProtoConvertible[T]](arr []P) []*T {
	protoSlice := make([]*T, len(arr))
	for i, item := range arr {
		protoSlice[i] = item.ToProto()
	}
	return protoSlice
}
