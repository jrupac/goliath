package admin

import (
	"context"
	"time"

	log "github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// redactedValue replaces a redacted string field when a request is logged.
const redactedValue = "[REDACTED]"

// auditLog records every admin call: what was asked, how it ended and how long
// it took. The service can add and delete users and change passwords, so what
// was done through it is worth being able to find afterwards.
func auditLog(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	rendered := "?"
	if m, ok := req.(proto.Message); ok {
		rendered = redacted(m)
	}
	log.Infof("Admin call %s {%s}: %s in %s", info.FullMethod, rendered, status.Code(err), time.Since(start))
	return resp, err
}

// redacted renders a message as text with every field marked debug_redact
// blanked out.
//
// The Go protobuf runtime does not act on that option when it formats a
// message, so a request printed any other way shows those fields as they
// are. This is the one place requests are rendered.
func redacted(m proto.Message) string {
	c := proto.Clone(m)
	redact(c.ProtoReflect())
	return prototext.MarshalOptions{}.Format(c)
}

func redact(m protoreflect.Message) {
	// Collected first rather than changed during the walk, which the
	// reflection API does not promise to tolerate.
	var sensitive []protoreflect.FieldDescriptor
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if opts, ok := fd.Options().(*descriptorpb.FieldOptions); ok && opts.GetDebugRedact() {
			sensitive = append(sensitive, fd)
			return true
		}
		if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
			return true
		}
		switch {
		case fd.IsList():
			for i := range v.List().Len() {
				redact(v.List().Get(i).Message())
			}
		case fd.IsMap():
			if k := fd.MapValue().Kind(); k == protoreflect.MessageKind || k == protoreflect.GroupKind {
				v.Map().Range(func(_ protoreflect.MapKey, mv protoreflect.Value) bool {
					redact(mv.Message())
					return true
				})
			}
		default:
			redact(v.Message())
		}
		return true
	})

	for _, fd := range sensitive {
		if fd.Kind() == protoreflect.StringKind && fd.Cardinality() != protoreflect.Repeated {
			m.Set(fd, protoreflect.ValueOfString(redactedValue))
		} else {
			m.Clear(fd)
		}
	}
}
