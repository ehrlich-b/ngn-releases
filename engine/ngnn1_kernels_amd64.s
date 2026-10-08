//go:build amd64 && amd64.v3 && !purego && !race

#include "textflag.h"

// NGN NGNN1 kernels, derived directly from its integer specification.
// int32 accumulators cover B1 plus 64 extreme int16 rows (< 2^22).
// Every AVX2 instruction uses unaligned or register operands.

TEXT ·ngnn1AddRow(SB), NOSPLIT, $0-48
	MOVQ acc_base+0(FP), DI
	MOVQ acc_base+0(FP), SI
	MOVQ row_base+24(FP), R8
	MOVQ row_len+32(FP), CX
AddRow_loop:
	VMOVDQU (SI), Y0
	VPMOVSXWD (R8), Y1
	VPADDD Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $16, R8
	SUBQ $8, CX
	JNZ AddRow_loop
	VZEROUPPER
	RET

TEXT ·ngnn1SubRow(SB), NOSPLIT, $0-48
	MOVQ acc_base+0(FP), DI
	MOVQ acc_base+0(FP), SI
	MOVQ row_base+24(FP), R8
	MOVQ row_len+32(FP), CX
SubRow_loop:
	VMOVDQU (SI), Y0
	VPMOVSXWD (R8), Y1
	VPSUBD Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $16, R8
	SUBQ $8, CX
	JNZ SubRow_loop
	VZEROUPPER
	RET

TEXT ·ngnn1MoveRows(SB), NOSPLIT, $0-96
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ remove_base+48(FP), R8
	MOVQ add_base+72(FP), R9
	MOVQ add_len+80(FP), CX
MoveRows_loop:
	VMOVDQU (SI), Y0
	VPMOVSXWD (R8), Y1
	VPSUBD Y1, Y0, Y0
	VPMOVSXWD (R9), Y1
	VPADDD Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $16, R8
	ADDQ $16, R9
	SUBQ $8, CX
	JNZ MoveRows_loop
	VZEROUPPER
	RET

TEXT ·ngnn1CaptureRows(SB), NOSPLIT, $0-120
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ remove_base+48(FP), R8
	MOVQ capture_base+72(FP), R9
	MOVQ add_base+96(FP), R10
	MOVQ add_len+104(FP), CX
CaptureRows_loop:
	VMOVDQU (SI), Y0
	VPMOVSXWD (R8), Y1
	VPSUBD Y1, Y0, Y0
	VPMOVSXWD (R9), Y1
	VPSUBD Y1, Y0, Y0
	VPMOVSXWD (R10), Y1
	VPADDD Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $16, R8
	ADDQ $16, R9
	ADDQ $16, R10
	SUBQ $8, CX
	JNZ CaptureRows_loop
	VZEROUPPER
	RET

TEXT ·ngnn1CastleRows(SB), NOSPLIT, $0-144
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ removeKing_base+48(FP), R8
	MOVQ removeRook_base+72(FP), R9
	MOVQ addKing_base+96(FP), R10
	MOVQ addRook_base+120(FP), R11
	MOVQ addRook_len+128(FP), CX
CastleRows_loop:
	VMOVDQU (SI), Y0
	VPMOVSXWD (R8), Y1
	VPSUBD Y1, Y0, Y0
	VPMOVSXWD (R9), Y1
	VPSUBD Y1, Y0, Y0
	VPMOVSXWD (R10), Y1
	VPADDD Y1, Y0, Y0
	VPMOVSXWD (R11), Y1
	VPADDD Y1, Y0, Y0
	VMOVDQU Y0, (DI)
	ADDQ $32, SI
	ADDQ $32, DI
	ADDQ $16, R8
	ADDQ $16, R9
	ADDQ $16, R10
	ADDQ $16, R11
	SUBQ $8, CX
	JNZ CastleRows_loop
	VZEROUPPER
	RET

// Wide-weight path. Each signed int32 product is exact; widen it before
// adding to any other product. The sum for both H=2048 perspectives is <2^44.
TEXT ·ngnn1Dot(SB), NOSPLIT, $0-56
	MOVQ acc_base+0(FP), SI
	MOVQ weights_base+24(FP), DI
	MOVQ weights_len+32(FP), CX
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	MOVL $255, AX
	MOVD AX, X7
	VPBROADCASTD X7, Y7
Dot_loop:
	VMOVDQU (SI), Y0
	VPMAXSD Y5, Y0, Y0
	VPMINSD Y7, Y0, Y0
	VPMULLD Y0, Y0, Y0
	VPMOVSXWD (DI), Y1
	VPMULLD Y1, Y0, Y0
	VPMOVSXDQ X0, Y2
	VEXTRACTI128 $1, Y0, X1
	VPMOVSXDQ X1, Y3
	VPADDQ Y2, Y6, Y6
	VPADDQ Y3, Y6, Y6
	ADDQ $32, SI
	ADDQ $16, DI
	SUBQ $8, CX
	JNZ Dot_loop
	VEXTRACTI128 $1, Y6, X0
	VPADDQ X0, X6, X6
	VPSHUFD $0x4e, X6, X0
	VPADDQ X0, X6, X6
	MOVQ X6, AX
	MOVQ AX, ret+48(FP)
	VZEROUPPER
	RET

// Packed path for loader-verified weights within [-128,128]. Clamp int32
// before packing. Reorder the lane-local pack into neuron order. x*w fits
// signed int16, so madd(x, x*w) gives exact int32 pair sums. Keep White/Black
// sums separate until sign extension: H/8 products per int32 lane, <=2^31-1.
TEXT ·ngnn1OutputSmall(SB), NOSPLIT, $0-80
	MOVQ us_base+0(FP), SI
	MOVQ them_base+24(FP), DI
	MOVQ weights_base+48(FP), R8
	MOVQ us_len+8(FP), CX
	LEAQ (R8)(CX*2), R9
	VPXOR Y5, Y5, Y5
	VPXOR Y6, Y6, Y6
	VPXOR Y7, Y7, Y7
	MOVL $255, AX
	MOVD AX, X4
	VPBROADCASTD X4, Y4
OutputSmall_loop:
	VMOVDQU (SI), Y0
	VMOVDQU 32(SI), Y1
	VPMAXSD Y5, Y0, Y0
	VPMAXSD Y5, Y1, Y1
	VPMINSD Y4, Y0, Y0
	VPMINSD Y4, Y1, Y1
	VPACKSSDW Y1, Y0, Y2
	VPERMQ $0xd8, Y2, Y2
	VPMULLW (R8), Y2, Y3
	VPMADDWD Y2, Y3, Y3
	VPADDD Y3, Y6, Y6
	VMOVDQU (DI), Y0
	VMOVDQU 32(DI), Y1
	VPMAXSD Y5, Y0, Y0
	VPMAXSD Y5, Y1, Y1
	VPMINSD Y4, Y0, Y0
	VPMINSD Y4, Y1, Y1
	VPACKSSDW Y1, Y0, Y2
	VPERMQ $0xd8, Y2, Y2
	VPMULLW (R9), Y2, Y3
	VPMADDWD Y2, Y3, Y3
	VPADDD Y3, Y7, Y7
	ADDQ $64, SI
	ADDQ $64, DI
	ADDQ $32, R8
	ADDQ $32, R9
	SUBQ $16, CX
	JNZ OutputSmall_loop
	VPMOVSXDQ X6, Y0
	VEXTRACTI128 $1, Y6, X1
	VPMOVSXDQ X1, Y1
	VPMOVSXDQ X7, Y2
	VEXTRACTI128 $1, Y7, X3
	VPMOVSXDQ X3, Y3
	VPADDQ Y1, Y0, Y0
	VPADDQ Y3, Y2, Y2
	VPADDQ Y2, Y0, Y0
	VEXTRACTI128 $1, Y0, X1
	VPADDQ X1, X0, X0
	VPSHUFD $0x4e, X0, X1
	VPADDQ X1, X0, X0
	MOVQ X0, AX
	MOVQ AX, ret+72(FP)
	VZEROUPPER
	RET
